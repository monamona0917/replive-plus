package dal

import (
	"context"
	"sync"
	"time"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gLog "gorm.io/gorm/logger"
)

type RecordMeta struct {
	Id        int64  `json:"id"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
	Creator   string `json:"creator"`
	Updater   string `json:"updater"`
}

var (
	db   *gorm.DB
	lock sync.Mutex
)

func ReadDB() *gorm.DB {
	return db
}

func DB() *gorm.DB {
	lock.Lock()
	return db
}

func ReleaseDB() {
	lock.Unlock()
}

func WithWriteDB(fn func(*gorm.DB) error) error {
	lock.Lock()
	defer lock.Unlock()
	return fn(db)
}

func InitDB() error {
	var err error
	db, err = gorm.Open(sqlite.Open("./sqlite.db"), &gorm.Config{
		Logger: &DLog{hlog.DefaultLogger()},
	})
	if err != nil {
		hlog.Errorf("open sqlite db err: %v", err)
		return err
	}
	if err = createTable(); err != nil {
		hlog.Errorf("createTable err: %v", err)
		return err
	}
	return nil
}

func CloseDB() error {
	lock.Lock()
	defer lock.Unlock()
	if db == nil {
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	db = nil
	return sqlDB.Close()
}

type DLog struct {
	Logger hlog.FullLogger
}

func (D DLog) LogMode(level gLog.LogLevel) gLog.Interface {
	return D
}

func (D DLog) Info(ctx context.Context, s string, i ...interface{}) {
	D.Logger.CtxInfof(ctx, s, i...)
}

func (D DLog) Warn(ctx context.Context, s string, i ...interface{}) {
	D.Logger.CtxWarnf(ctx, s, i...)
}

func (D DLog) Error(ctx context.Context, s string, i ...interface{}) {
	D.Logger.CtxErrorf(ctx, s, i...)
}

func (D DLog) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
}
