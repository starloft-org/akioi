package service

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"akioi/internal/config"
	"gorm.io/gorm"
)

// backupNameRe 限定备份文件名，避免用文件名做目录穿越。
var backupNameRe = regexp.MustCompile(`^akioi-\d{8}-\d{6}\.sql\.gz$`)

// backupBinaryTypes 是导出时需要写成十六进制字面量的列类型。
var backupBinaryTypes = map[string]bool{
	"BINARY":     true,
	"VARBINARY":  true,
	"TINYBLOB":   true,
	"BLOB":       true,
	"MEDIUMBLOB": true,
	"LONGBLOB":   true,
	"BIT":        true,
}

// BackupFile 是备份目录里的一个备份文件。
type BackupFile struct {
	Name      string    `json:"name"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
}

// BackupService 用纯 Go 生成 MySQL 逻辑备份，不依赖 mysqldump。
type BackupService struct {
	db  *gorm.DB
	dir string
	mu  sync.Mutex
}

// NewBackup 创建备份服务；dir 为空时退回 /app/backups。
func NewBackup(db *gorm.DB, cfg *config.Config) *BackupService {
	dir := "/app/backups"
	if cfg != nil && strings.TrimSpace(cfg.BackupDir) != "" {
		dir = strings.TrimSpace(cfg.BackupDir)
	}
	return &BackupService{db: db, dir: dir}
}

// Dir 返回备份目录。
func (s *BackupService) Dir() string {
	return s.dir
}

// List 返回按文件名倒序排列的备份列表。
func (s *BackupService) List() ([]BackupFile, error) {
	if err := os.MkdirAll(s.dir, 0o750); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	files := make([]BackupFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !backupNameRe.MatchString(entry.Name()) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, BackupFile{Name: entry.Name(), Size: info.Size(), CreatedAt: info.ModTime()})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name > files[j].Name })
	return files, nil
}

// Path 返回备份文件的绝对路径，文件名不合法时报错。
func (s *BackupService) Path(name string) (string, error) {
	if !backupNameRe.MatchString(name) {
		return "", fmt.Errorf("备份文件名不合法")
	}
	return filepath.Join(s.dir, name), nil
}

// Delete 删除一个备份文件。
func (s *BackupService) Delete(name string) error {
	path, err := s.Path(name)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("备份文件不存在")
		}
		return err
	}
	return nil
}

// Create 导出当前数据库到 data/backups 下的 gzip 压缩 SQL 文件。
func (s *BackupService) Create() (BackupFile, error) {
	if s.db == nil {
		return BackupFile{}, fmt.Errorf("数据库连接不可用")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(s.dir, 0o750); err != nil {
		return BackupFile{}, err
	}
	now := time.Now()
	name := fmt.Sprintf("akioi-%s.sql.gz", now.Format("20060102-150405"))
	final := filepath.Join(s.dir, name)
	temp := final + ".tmp"

	file, err := os.OpenFile(temp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o640)
	if err != nil {
		return BackupFile{}, err
	}
	writer := bufio.NewWriterSize(file, 256*1024)
	gz := gzip.NewWriter(writer)

	writeErr := s.writeDump(gz, now)
	if writeErr == nil {
		writeErr = gz.Close()
	} else {
		_ = gz.Close()
	}
	if writeErr == nil {
		writeErr = writer.Flush()
	}
	if closeErr := file.Close(); writeErr == nil {
		writeErr = closeErr
	}
	if writeErr != nil {
		_ = os.Remove(temp)
		return BackupFile{}, writeErr
	}
	if err := os.Rename(temp, final); err != nil {
		_ = os.Remove(temp)
		return BackupFile{}, err
	}
	info, err := os.Stat(final)
	if err != nil {
		return BackupFile{}, err
	}
	return BackupFile{Name: name, Size: info.Size(), CreatedAt: info.ModTime()}, nil
}

// writeDump 按表写出建表语句与全部数据。
func (s *BackupService) writeDump(gz *gzip.Writer, now time.Time) error {
	var head strings.Builder
	head.WriteString("-- AKIOI 数据库备份\n")
	head.WriteString(fmt.Sprintf("-- 生成时间: %s\n", now.Format("2006-01-02 15:04:05")))
	head.WriteString("-- 恢复方式: gunzip -c 该文件 | mysql -h 主机 -u 用户 -p 库名\n\n")
	head.WriteString("SET NAMES utf8mb4;\n")
	head.WriteString("SET @OLD_FOREIGN_KEY_CHECKS=@@FOREIGN_KEY_CHECKS, FOREIGN_KEY_CHECKS=0;\n")
	head.WriteString("SET @OLD_UNIQUE_CHECKS=@@UNIQUE_CHECKS, UNIQUE_CHECKS=0;\n\n")
	if _, err := gz.Write([]byte(head.String())); err != nil {
		return err
	}

	tables, err := s.tableNames()
	if err != nil {
		return err
	}
	for _, table := range tables {
		if err := s.writeTable(gz, table); err != nil {
			return err
		}
	}

	foot := "\nSET UNIQUE_CHECKS=@OLD_UNIQUE_CHECKS;\nSET FOREIGN_KEY_CHECKS=@OLD_FOREIGN_KEY_CHECKS;\n"
	_, err = gz.Write([]byte(foot))
	return err
}

// tableNames 返回当前库的所有表名。
func (s *BackupService) tableNames() ([]string, error) {
	rows, err := s.db.Raw("SHOW TABLES").Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	return tables, rows.Err()
}

// writeTable 写出一张表的建表语句与数据。
func (s *BackupService) writeTable(gz *gzip.Writer, table string) error {
	var created, ddl string
	if err := s.db.Raw("SHOW CREATE TABLE "+sqlIdentifier(table)).Row().Scan(&created, &ddl); err != nil {
		return fmt.Errorf("读取表 %s 的结构失败: %w", table, err)
	}
	var head strings.Builder
	head.WriteString(fmt.Sprintf("--\n-- 表 %s\n--\n", table))
	head.WriteString("DROP TABLE IF EXISTS " + sqlIdentifier(table) + ";\n")
	head.WriteString(ddl + ";\n\n")
	if _, err := gz.Write([]byte(head.String())); err != nil {
		return err
	}

	rows, err := s.db.Raw("SELECT * FROM " + sqlIdentifier(table)).Rows()
	if err != nil {
		return fmt.Errorf("读取表 %s 的数据失败: %w", table, err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return err
	}
	if len(columns) == 0 {
		return nil
	}
	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return err
	}
	binary := make([]bool, len(columns))
	for i, ct := range columnTypes {
		if i < len(binary) {
			binary[i] = backupBinaryTypes[strings.ToUpper(ct.DatabaseTypeName())]
		}
	}

	prefix := "INSERT INTO " + sqlIdentifier(table) + " (" + joinIdentifiers(columns) + ") VALUES\n"
	values := make([]any, len(columns))
	pointers := make([]any, len(columns))
	for i := range values {
		pointers[i] = &values[i]
	}

	var batch strings.Builder
	batch.WriteString(prefix)
	pending := 0
	for rows.Next() {
		if err := rows.Scan(pointers...); err != nil {
			return err
		}
		if pending > 0 {
			batch.WriteString(",\n")
		}
		batch.WriteString("(")
		for i := range values {
			if i > 0 {
				batch.WriteString(",")
			}
			batch.WriteString(sqlLiteral(values[i], binary[i]))
		}
		batch.WriteString(")")
		pending++
		// 控制单条 INSERT 的体积，避免内存暴涨。
		if pending >= 500 || batch.Len() >= 512*1024 {
			batch.WriteString(";\n")
			if _, err := gz.Write([]byte(batch.String())); err != nil {
				return err
			}
			batch.Reset()
			batch.WriteString(prefix)
			pending = 0
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if pending > 0 {
		batch.WriteString(";\n")
		if _, err := gz.Write([]byte(batch.String())); err != nil {
			return err
		}
	}
	_, err = gz.Write([]byte("\n"))
	return err
}

// sqlIdentifier 用反引号包裹表名或列名，并转义内部的反引号。
func sqlIdentifier(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

// joinIdentifiers 拼接多个列名。
func joinIdentifiers(names []string) string {
	parts := make([]string, len(names))
	for i, name := range names {
		parts[i] = sqlIdentifier(name)
	}
	return strings.Join(parts, ",")
}

// sqlLiteral 把一列的值渲染成 SQL 字面量；二进制类型按十六进制输出，避免字符集转换。
func sqlLiteral(value any, binary bool) string {
	switch v := value.(type) {
	case nil:
		return "NULL"
	case time.Time:
		return "'" + v.Format("2006-01-02 15:04:05.999999") + "'"
	case []byte:
		if binary {
			if len(v) == 0 {
				return "''"
			}
			return "0x" + fmt.Sprintf("%x", v)
		}
		return sqlQuote(string(v))
	case string:
		if binary {
			return "0x" + fmt.Sprintf("%x", []byte(v))
		}
		return sqlQuote(v)
	case bool:
		if v {
			return "1"
		}
		return "0"
	default:
		return fmt.Sprintf("%v", v)
	}
}

// sqlQuote 转义字符串里的反斜杠、引号与控制字符。
func sqlQuote(text string) string {
	var b strings.Builder
	b.Grow(len(text) + 2)
	b.WriteByte('\'')
	for i := 0; i < len(text); i++ {
		switch c := text[i]; c {
		case '\\':
			b.WriteString(`\\`)
		case '\'':
			b.WriteString(`\'`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case 0x00:
			b.WriteString(`\0`)
		case 0x1a:
			b.WriteString(`\Z`)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteByte('\'')
	return b.String()
}
