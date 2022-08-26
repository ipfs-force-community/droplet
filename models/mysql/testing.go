package mysql

import (
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/filecoin-project/venus-market/v2/models/repo"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func setup(t *testing.T) (repo.Repo, sqlmock.Sqlmock, *sql.DB) {
	sqlDB, mock, err := sqlmock.New()
	assert.NoError(t, err)

	mock.ExpectQuery("SELECT VERSION()").WithArgs().
		WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow(""))

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn: sqlDB,
	}))
	assert.NoError(t, err)

	return MysqlRepo{DB: gormDB}, mock, sqlDB
}
