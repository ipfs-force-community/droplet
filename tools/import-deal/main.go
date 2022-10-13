package main

import (
	"flag"
	"fmt"
	"sync"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/filecoin-project/venus-market/v2/tools/import-deal/types"
)

func ImportDealsToMysql(srcConn, conn string, nums int) error {
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

	idx := 0
	fmt.Printf("has deals: %v.\n", len(deals))
	wg := sync.WaitGroup{}
	for {
		if idx >= len(deals) {
			break
		}
		end := idx + nums
		if end > len(deals) {
			end = len(deals)
		}

		wg.Add(1)
		go func(start, end int) {
			defer wg.Done()

			err := dstDb.Create(deals[start:end]).Error
			if err != nil {
				fmt.Printf("import [%d, %d) records error: %s\n", start, end, err.Error())
			} else {
				fmt.Printf("import [%d, %d) records success\n", start, end)
			}
		}(idx, end)

		idx += nums
	}
	wg.Wait()
	fmt.Println("import records end")

	return nil
}

func main() {
	// mysql: user:password@tcp(localhost:3308)/db-name?loc=Local&parseTime=true&innodb_lock_wait_timeout=10
	var (
		srcConn, conn string
		nums          int
	)

	flag.StringVar(&srcConn, "src-conn", "", "mysql conn for src")
	flag.StringVar(&conn, "conn", "", "mysql conn for market")
	flag.IntVar(&nums, "nums", 50, "The number of imports each time")

	flag.Parse()

	if err := ImportDealsToMysql(srcConn, conn, nums); err != nil {
		fmt.Println("import err: ", err.Error())
	}

	fmt.Println("import success.")
}
