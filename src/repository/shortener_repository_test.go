// package repository

// import (
// 	"context"
// 	"testing"

// 	"github.com/stretchr/testify/mock"
// )

// type MockDB struct {
// 	mock.Mock
// }

// func (m *MockDB) Insert(data model.Data) error {
//     // Mock insert operation
//     args := m.Called(data)
//     return args.Error(0)
// }

// func Test_shortenerRepository_SaveShotenedURL(t *testing.T) {
// 	tests := []struct {
// 		name string
// 		cache map[string]string
// 		url   string
// 		shortenedUrl string
// 		want   error
// 	}{
// 		{"key does not exist", map[string]string{"key1": "value1"}, "www.test.com", "WzadtdDb",  nil},
// 		{"key exists", map[string]string{"Rgdt4Gt5": "www.test2.com"}, "www.test2.com", "Rgdt4Gt5",  nil},
// 	}

// 	context := context.TODO()

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			sr := &shortenerRepository{
// 				Cache: tt.cache,
// 			}
// 			if got := sr.SaveShotenedURL(context, tt.url, tt.shortenedUrl); got != tt.want {
// 				t.Errorf("shortenerRepository.SaveShotenedURL() = %v, want %v", got, tt.want)
// 			}
// 		})
// 	}
// }

// func Test_shortenerRepository_GetOriginalURL(t *testing.T) {
// 	context := context.TODO()
// 	tests := []struct {
// 		name string
// 		cache map[string]string
// 		shortenedUrl string
// 		want   string
// 	}{
// 		{"key does not exist", map[string]string{"key1": "value1"},  "WzadtdDb",  ""},
// 		{"key exists", map[string]string{"Rgdt4Gt5": "www.test2.com"}, "Rgdt4Gt5",  "www.test2.com"},
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			sr := &shortenerRepository{
// 				Cache: tt.cache,
// 			}
// 			if got, _ := sr.GetOriginalURL(context, tt.shortenedUrl); got != tt.want {
// 				t.Errorf("shortenerRepository.GetOriginalURL() = %v, want %v", got, tt.want)
// 			}
// 		})
// 	}
// }

package repository

// import (
// 	"context"
// 	"database/sql"
// 	"testing"

// 	"github.com/stretchr/testify/assert"
// 	"github.com/stretchr/testify/mock"
// )

// // MockDB is a mock database interface
// type MockDB struct {
//     mock.Mock
// }

// func (m *MockDB) Prepare(query string) (*sql.Stmt, error) {
//     args := m.Called(query)
//     return args.Get(0).(*sql.Stmt), args.Error(1)
// }

// func (m *MockDB) Exec(query string, args ...interface{}) (sql.Result, error) {
//     calledArgs := m.Called(query, args)
//     return calledArgs.Get(0).(sql.Result), calledArgs.Error(1)
// }

// func (m *MockDB) QueryRow(query string, args ...interface{}) *sql.Row {
//     calledArgs := m.Called(query, args)
//     return calledArgs.Get(0).(*sql.Row)
// }

// func TestSaveShortenedURL(t *testing.T) {
//     // Create mock database
//     mockDB := new(MockDB)
//     repo := NewShortenerRepository(mockDB)

//     // Prepare mock statements
//     stmtMock := new(sql.Stmt)
//     mockDB.On("Prepare", "INSERT INTO URLS (LONG_URL, SHORT_URL, COUNTER_VALUE) VALUES ($1, $2, $3)").Return(stmtMock, nil)

//     // Expectations for Exec method
//     resultMock := new(sql.Result)
//     mockDB.On("Exec", "INSERT INTO URLS (LONG_URL, SHORT_URL, COUNTER_VALUE) VALUES ($1, $2, $3)", "test_long_url", "test_short_url", 1).Return(resultMock, nil)

//     // Call repository method
//     err := repo.SaveShotenedURL(context.Background(), "test_long_url", "test_short_url")

//     // Assertions
//     assert.NoError(t, err)
//     mockDB.AssertExpectations(t)
// }

// func TestGetOriginalURL(t *testing.T) {
//     // Create mock database
//     mockDB := new(MockDB)
//     repo := NewShortenerRepository(mockDB)

//     // Prepare mock rows and result
//     rowMock := new(sql.Row)
//     resultMock := new(sql.Result)
//     mockDB.On("QueryRow", "SELECT LONG_URL FROM URLS WHERE SHORT_URL = $1", "test_short_url").Return(rowMock)
//     rowMock.Scan("Scan", mock.AnythingOfType("*string")).Return(nil)
//     resultMock.On("RowsAffected").Return(1)

//     // Call repository method
//     _, err := repo.GetOriginalURL(context.Background(), "test_short_url")

//     // Assertions
//     assert.NoError(t, err)
//     mockDB.AssertExpectations(t)
// }

// func TestIfURLAlreadyExists(t *testing.T) {
//     // Create mock database
//     mockDB := new(MockDB)
//     repo := NewShortenerRepository(mockDB)

//     // Prepare mock rows and result
//     rowMock := new(sql.Row)
//     resultMock := new(sql.Result)
//     mockDB.On("QueryRow", "SELECT SHORT_URL FROM URLS WHERE LONG_URL = $1", "test_long_url").Return(rowMock)
//     rowMock.On("Scan", mock.AnythingOfType("*string")).Return(nil)
//     resultMock.On("RowsAffected").Return(1)

//     // Call repository method
//     _, err := repo.IfURLAlreadyExists(context.Background(), "test_long_url")

//     // Assertions
//     assert.NoError(t, err)
//     mockDB.AssertExpectations(t)
// }
