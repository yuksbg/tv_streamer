package helpers

import (
	"fmt"
	"log"
	"os"
	"time"
	"tv_streamer/helpers/logs"
	"tv_streamer/migrations"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"xorm.io/xorm"
)

var engine *xorm.Engine

func GetXORM() *xorm.Engine {
	var err error
	if engine == nil {
		if os.Getenv("DB_PATH") == "" {
			os.Setenv("DB_PATH", GetConfig().Database.DBPath)
		}
		dbFile := fmt.Sprintf("%s/database.db", os.Getenv("DB_PATH"))
		logs.GetLogger().WithField(`path`, dbFile).Info(`loaded db path`)

		// _txlock=immediate
		engine, err = xorm.NewEngine("sqlite3", fmt.Sprintf("file:%s?_foreign_keys=on&_journal_mode=WAL&_cache_size=10000&_busy_timeout=5000", dbFile))
		if err != nil {
			log.Panicln(err.Error())
		}
		engine.ShowSQL(true)
		engine.SetMaxIdleConns(1)
		engine.SetMaxOpenConns(100)
		engine.SetConnMaxLifetime(10 * time.Minute)
		engine.SetConnMaxIdleTime(10 * time.Second)
		engine.Exec(`PRAGMA foreign_keys = ON`)
		engine.Exec(`PRAGMA journal_mode = WAL`)

		// Run database migrations
		sqlDB := engine.DB().DB
		if err := migrations.Run(sqlDB); err != nil {
			log.Panicln("Failed to run migrations:", err.Error())
		}
	}
	return engine
}
