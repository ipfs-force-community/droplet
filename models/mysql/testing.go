package mysql

import (
	"crypto/rand"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"reflect"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/filecoin-project/venus-market/v2/models/repo"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multihash"

	"github.com/stretchr/testify/assert"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/libp2p/go-libp2p-core/crypto"
	ptest "github.com/libp2p/go-libp2p-core/test"
)

func setup(t *testing.T) (repo.Repo, sqlmock.Sqlmock, *sql.DB) {
	sqlDB, mock, err := sqlmock.New()
	assert.NoError(t, err)

	mock.ExpectQuery("SELECT VERSION()").WithArgs().
		WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow(""))

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn: sqlDB,
	}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	assert.NoError(t, err)

	return MysqlRepo{DB: gormDB}, mock, sqlDB
}

func wrapper(f func(*testing.T, repo.Repo, sqlmock.Sqlmock), repo repo.Repo, mock sqlmock.Sqlmock) func(t *testing.T) {
	return func(t *testing.T) {
		f(t, repo, mock)
	}
}

func closeDB(mock sqlmock.Sqlmock, sqlDB *sql.DB) error {
	mock.ExpectClose()
	return sqlDB.Close()
}

func getTestCid() (cid.Cid, error) {
	temp := make([]byte, 8)
	_, err := rand.Read(temp)
	hash, err := multihash.Sum(temp, multihash.SHA3, -1)
	if err != nil {
		return cid.Undef, err
	}
	return cid.NewCidV1(cid.Raw, hash), nil
}

func getTestPeerId() (peer.ID, error) {
	sk, _, err := ptest.RandTestKeyPair(crypto.RSA, 2048)
	if err != nil {
		return "", err
	}
	peerId, err := peer.IDFromPrivateKey(sk)
	if err != nil {
		return "", err
	}
	return peerId, nil
}

func getDryrunDB() (*gorm.DB, error) {
	return gorm.Open(sqlite.Open(":memory:"), &gorm.Config{DryRun: true})
}

func getFullRows(values []interface{}) (*sqlmock.Rows, error) {
	if len(values) <= 0 {
		return nil, fmt.Errorf("values is empty")
	}

	db, err := getDryrunDB()
	if err != nil {
		return nil, err
	}

	err = db.Statement.Parse(values[0])
	if err != nil {
		return nil, err
	}

	schema := db.Statement.Schema
	rows := sqlmock.NewRows(schema.DBNames)
	dict := schema.FieldsByDBName

	for _, stru := range values {
		row := make([]driver.Value, 0, len(schema.DBNames))
		rt := reflect.TypeOf(stru)
		rv := reflect.ValueOf(stru)

		if rt.Kind() == reflect.Ptr {
			rt = rt.Elem()
			rv = rv.Elem()
		}

		if rt.Kind() != reflect.Struct {
			return nil, fmt.Errorf("value is not struct")
		}

		for _, dbName := range schema.DBNames {
			fiel := dict[dbName]
			temp := rv
			for _, path := range fiel.BindNames {
				temp = rv.FieldByName(path)
			}

			tt := temp.Type()

			if tt == reflect.TypeOf(driver.Valuer(nil)) {
				v, err := temp.Interface().(driver.Valuer).Value()
				if err != nil {
					return nil, err
				}
				// if fiel.DataType ==
				row = append(row, v.([]byte))
			} else {
				row = append(row, temp.Interface())
			}
		}

		rows.AddRow(row...)
	}
	return rows, nil
}

func getSQL(db *gorm.DB) (sql string, vars []driver.Value, err error) {
	stmt := db.Statement
	sql = stmt.SQL.String()
	varsI := stmt.Vars

	vars = make([]driver.Value, 0, len(varsI))
	for _, v := range varsI {
		vars = append(vars, v)
	}

	return sql, vars, nil
}
