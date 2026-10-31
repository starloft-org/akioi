package service

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"akioi/internal/config"
)

func TestSQLQuote(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "普通文本", in: "hello", want: "'hello'"},
		{name: "单引号", in: "it's", want: `'it\'s'`},
		{name: "反斜杠", in: `a\b`, want: `'a\\b'`},
		{name: "换行", in: "a\nb", want: `'a\nb'`},
		{name: "回车", in: "a\rb", want: `'a\rb'`},
		{name: "NUL 与 DOS 结束符", in: "a\x00b\x1a", want: `'a\0b\Z'`},
		{name: "空串", in: "", want: "''"},
	}
	for _, tc := range cases {
		if got := sqlQuote(tc.in); got != tc.want {
			t.Fatalf("%s: sqlQuote(%q) = %q, want %q", tc.name, tc.in, got, tc.want)
		}
	}
}

func TestSQLLiteral(t *testing.T) {
	moment := time.Date(2024, 5, 6, 7, 8, 9, 123456000, time.UTC)
	cases := []struct {
		name   string
		value  any
		binary bool
		want   string
	}{
		{name: "NULL", value: nil, want: "NULL"},
		{name: "时间", value: moment, want: "'2024-05-06 07:08:09.123456'"},
		{name: "文本字节", value: []byte("ok"), want: "'ok'"},
		{name: "二进制", value: []byte{0x00, 0xff}, binary: true, want: "0x00ff"},
		{name: "空二进制", value: []byte{}, binary: true, want: "''"},
		{name: "字符串列按二进制", value: "a", binary: true, want: "0x61"},
		{name: "布尔真", value: true, want: "1"},
		{name: "布尔假", value: false, want: "0"},
		{name: "整数", value: int64(42), want: "42"},
	}
	for _, tc := range cases {
		if got := sqlLiteral(tc.value, tc.binary); got != tc.want {
			t.Fatalf("%s: sqlLiteral() = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestBackupPathRejectsTraversal(t *testing.T) {
	svc := NewBackup(nil, &config.Config{BackupDir: t.TempDir()})
	badNames := []string{"../akioi-20240101-000000.sql.gz", "akioi-20240101-000000.sql", "other.sql.gz", "akioi-20240101-000000.sql.gz/../x"}
	for _, name := range badNames {
		if _, err := svc.Path(name); err == nil {
			t.Fatalf("Path(%q) 应当报错", name)
		}
	}
	if err := svc.Delete("../x"); err == nil {
		t.Fatal("Delete 应当拒绝非法文件名")
	}
}

func TestBackupListAndDelete(t *testing.T) {
	dir := t.TempDir()
	svc := NewBackup(nil, &config.Config{BackupDir: dir})
	names := []string{"akioi-20240101-010101.sql.gz", "akioi-20240202-020202.sql.gz"}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("dump"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("ignore"), 0o600); err != nil {
		t.Fatal(err)
	}
	files, err := svc.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != len(names) {
		t.Fatalf("List 返回 %d 个文件，期望 %d 个", len(files), len(names))
	}
	if files[0].Name != names[1] {
		t.Fatalf("List 期望按文件名倒序，首项为 %s，实际 %s", names[1], files[0].Name)
	}
	if err := svc.Delete(names[0]); err != nil {
		t.Fatal(err)
	}
	files, err = svc.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != len(names)-1 {
		t.Fatalf("删除后应剩 %d 个文件，实际 %d 个", len(names)-1, len(files))
	}
	if err := svc.Delete(names[0]); err == nil {
		t.Fatal("重复删除应当报错")
	}
}

func TestBackupCreateRequiresDatabase(t *testing.T) {
	svc := NewBackup(nil, &config.Config{BackupDir: t.TempDir()})
	if _, err := svc.Create(); err == nil {
		t.Fatal("数据库不可用时 Create 应当报错")
	}
}

func TestSQLIdentifier(t *testing.T) {
	if got := sqlIdentifier("problem_banks"); got != "`problem_banks`" {
		t.Fatalf("sqlIdentifier = %q", got)
	}
	if got := sqlIdentifier("we`ird"); got != "`we``ird`" {
		t.Fatalf("sqlIdentifier 未转义反引号: %q", got)
	}
	if got := joinIdentifiers([]string{"id", "name"}); got != "`id`,`name`" {
		t.Fatalf("joinIdentifiers = %q", got)
	}
}
