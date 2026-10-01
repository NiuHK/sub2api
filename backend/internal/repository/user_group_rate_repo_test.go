package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestSyncGroupRateMultipliersRollsBackOnUpsertFailure(t *testing.T) {
	rate1, rate2 := 1.1, 1.2
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec("SELECT pg_advisory_xact_lock").WithArgs(int64(17)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE user_group_rate_multipliers").
		WithArgs(int64(17), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM user_group_rate_multipliers").
		WithArgs(int64(17)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO user_group_rate_multipliers").
		WithArgs(sqlmock.AnyArg(), int64(17), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO user_group_rate_multipliers").
		WithArgs(sqlmock.AnyArg(), int64(17), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnError(errors.New("injected upsert failure"))
	mock.ExpectRollback()

	repo := NewUserGroupRateRepository(db)
	err = repo.SyncGroupRateMultipliers(context.Background(), 17, []service.GroupRateMultiplierInput{
		{UserID: 101, RateMultiplier: &rate1},
		{UserID: 102, RateMultiplier: &rate2},
	})
	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSyncGroupRateMultipliersCommitsAllWrites(t *testing.T) {
	rate := 0.8
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec("SELECT pg_advisory_xact_lock").WithArgs(int64(23)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE user_group_rate_multipliers").
		WithArgs(int64(23), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM user_group_rate_multipliers").
		WithArgs(int64(23)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO user_group_rate_multipliers").
		WithArgs(sqlmock.AnyArg(), int64(23), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	repo := NewUserGroupRateRepository(db)
	err = repo.SyncGroupRateMultipliers(context.Background(), 23, []service.GroupRateMultiplierInput{
		{UserID: 201, RateMultiplier: &rate},
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

var _ interface {
	BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error)
} = (*sql.DB)(nil)
