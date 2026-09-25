// Package migrate 执行 golang-migrate 版本化迁移。
package migrate

import (
	"database/sql"
	"errors"
	"fmt"
	"os"

	"akioi/migrations"

	"github.com/golang-migrate/migrate/v4"
	migratemysql "github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/source"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// Run 执行全部未应用的迁移，已是最新时返回 nil。
func Run(dsn string) error {
	src, m, closeAll, err := open(dsn)
	if err != nil {
		return err
	}
	defer closeAll()

	_ = src
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("执行数据库迁移失败: %w", err)
	}
	return nil
}

// Pending 返回未应用的迁移版本号与当前已应用版本。
func Pending(dsn string) ([]uint, uint, error) {
	src, m, closeAll, err := open(dsn)
	if err != nil {
		return nil, 0, err
	}
	defer closeAll()

	var current uint
	version, dirty, err := m.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return nil, 0, err
	}
	if dirty {
		return nil, version, fmt.Errorf("数据库迁移处于 dirty 状态，版本 %d", version)
	}
	current = version

	all, err := sourceVersions(src)
	if err != nil {
		return nil, current, err
	}
	var pending []uint
	for _, v := range all {
		if v > current {
			pending = append(pending, v)
		}
	}
	return pending, current, nil
}

func sourceVersions(d source.Driver) ([]uint, error) {
	var out []uint
	v, err := d.First()
	for err == nil {
		out = append(out, v)
		v, err = d.Next(v)
	}
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	return out, err
}

// open 构造迁移实例并返回统一的关闭函数。
func open(dsn string) (source.Driver, *migrate.Migrate, func(), error) {
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return nil, nil, nil, err
	}
	sqlDB, err := sql.Open("mysql", dsn)
	if err != nil {
		_ = src.Close()
		return nil, nil, nil, err
	}
	driver, err := migratemysql.WithInstance(sqlDB, &migratemysql.Config{})
	if err != nil {
		_ = src.Close()
		_ = sqlDB.Close()
		return nil, nil, nil, err
	}
	m, err := migrate.NewWithInstance("iofs", src, "mysql", driver)
	if err != nil {
		_ = src.Close()
		_ = sqlDB.Close()
		return nil, nil, nil, err
	}
	closeAll := func() {
		_, _ = m.Close()
	}
	return src, m, closeAll, nil
}
