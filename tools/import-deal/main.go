package main

import (
	"flag"
	"fmt"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/filecoin-project/venus-market/v2/tools/import-deal/types"
)

func ImportDealsToMysql(srcConn, conn string) error {
	var (
		maxOpenConn = 10
		maxIdleConn = 10
	)

	db, err := gorm.Open(mysql.Open(srcConn))
	if err != nil {
		return err
	}

	db.Set("gorm:table_options", "CHARSET=utf8mb4")
	db = db.Debug()

	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	defer func() {
		_ = sqlDB.Close()
	}()

	sqlDB.SetMaxOpenConns(maxOpenConn)
	sqlDB.SetMaxIdleConns(maxIdleConn)
	sqlDB.SetConnMaxLifetime(30 * time.Second)

	forceDeals := []*types.ForceDeal{}
	if err := db.Table("deals").Find(&forceDeals).Error; err != nil {
		return err
	}

	// venus-market deals
	dstDb, err := gorm.Open(mysql.Open(conn))
	if err != nil {
		return err
	}

	dstDb.Set("gorm:table_options", "CHARSET=utf8mb4")
	dstDb = dstDb.Debug()

	dstSqlDB, err := dstDb.DB()
	if err != nil {
		return err
	}
	defer func() {
		_ = dstSqlDB.Close()
	}()

	dstSqlDB.SetMaxOpenConns(maxOpenConn)
	dstSqlDB.SetMaxIdleConns(maxIdleConn)
	dstSqlDB.SetConnMaxLifetime(30 * time.Second)

	deals := make([]types.Deal, 0, len(forceDeals))
	for _, deal := range forceDeals {
		deals = append(deals, *deal.ToDeal())
	}

	if err = dstDb.AutoMigrate(&types.Deal{}); err != nil {
		return err
	}

	return dstDb.Create(&deals).Error
}

func main() {
	// mysql: user:password@tcp(localhost:3308)/db-name?loc=Local&parseTime=true&innodb_lock_wait_timeout=10
	var (
		srcConn, conn string
	)

	flag.StringVar(&srcConn, "src-conn", "", "mysql conn for src")
	flag.StringVar(&conn, "conn", "", "mysql conn for market")

	flag.Parse()

	if err := ImportDealsToMysql(srcConn, conn); err != nil {
		fmt.Printf("import deals to mysql err: %s\n", err.Error())
		return
	}

	fmt.Println("import success.")
}
