package database

import (
	"database/sql"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
)

func Init(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)

	if err != nil {
		return nil, err
	}

	if !tablesExist(db) {
		log.Println("База данных не найдена, создаём новую...")
		if err := createTables(db); err != nil {
			return nil, err
		}
		log.Println("Таблицы успешно созданы")
	} else {
		log.Println("База данных подключена")
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}

func tablesExist(db *sql.DB) bool {
	tables := []string{"users", "letters", "user_letters"}

	for _, table := range tables {
		var name string
		query := "SELECT name FROM sqlite_master WHERE type='table' AND name=?"
		err := db.QueryRow(query, table).Scan(&name)
		if err != nil || name != table {
			return false
		}
	}
	return true
}

func createTables(db *sql.DB) error {
	queries := []string{
		`
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,  
			name TEXT NOT NULL UNIQUE              
		);

		CREATE TABLE IF NOT EXISTS letters (
			id INTEGER PRIMARY KEY AUTOINCREMENT,  
			char TEXT NOT NULL UNIQUE              
		);

		CREATE TABLE IF NOT EXISTS user_letters (
			user_id INTEGER,                      
			letter_id INTEGER,                    
			PRIMARY KEY (user_id, letter_id),     
			FOREIGN KEY (user_id) REFERENCES users(id),
			FOREIGN KEY (letter_id) REFERENCES letters(id)
		);`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return err
		}
	}

	return nil
}

func GetUserID(db *sql.DB, userName string) (int, error) {
	var id int
	err := db.QueryRow("SELECT id FROM users WHERE name = ?", userName).Scan(&id)
	return id, err
}

func Grant(db *sql.DB, UserID int, LetterID int) error {
	_, err := db.Exec(
		"INSERT OR IGNORE INTO user_letters (user_id, letter_id) VALUES (?, ?)",
		UserID, LetterID,
	)
	return err
}

func CreateUser(db *sql.DB, name string) (UserID int, err error) {
	_, err = db.Exec("INSERT OR IGNORE INTO users (name) VALUES (?)", name)
	if err != nil {
		return 0, err
	}

	UserID, err = GetUserID(db, name)
	return UserID, err
}

func CreateLetter(db *sql.DB, letter rune) (LetterID int, err error) {
	letterStr := string(letter)
	_, err = db.Exec("INSERT OR IGNORE INTO letters (char) VALUES (?)", letterStr)
	if err != nil {
		return 0, err
	}
	LetterID, err = GetLetterID(db, letter)
	return LetterID, err
}

func Create(db *sql.DB, name string, letters ...rune) error {
	UserID, err := CreateUser(db, name)
	if err != nil {
		return err
	}

	for _, letter := range letters {
		LetterID, err := CreateLetter(db, letter)
		if err != nil {
			return err
		}

		if err := Grant(db, UserID, LetterID); err != nil {
			return err
		}
	}

	return nil
}

func Remove(db *sql.DB, UserID int, LetterID int) error {
	_, err := db.Exec(
		"DELETE FROM user_letters WHERE user_id = ? AND letter_id = ?",
		UserID, LetterID,
	)
	return err
}

func GrantAll(db *sql.DB, UserID int) error {
	_, err := db.Exec(
		`INSERT OR IGNORE INTO user_letters (user_id, letter_id)
         SELECT ?, id FROM letters`,
		UserID,
	)
	return err
}

func RemoveAll(db *sql.DB, UserID int) error {
	_, err := db.Exec("DELETE FROM user_letters WHERE user_id = ?", UserID)
	return err
}

func FindUser(db *sql.DB, userName string) (UserID int, err error) {
	var id int
	err = db.QueryRow("SELECT id FROM users WHERE name = ?", userName).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func GetPermissions(db *sql.DB, UserID int) (AccessableLetters []string, err error) {
	rows, err := db.Query(`
        SELECT l.char 
        FROM letters l
        JOIN user_letters ul ON l.id = ul.letter_id
        WHERE ul.user_id = ?
    `, UserID)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var letters []string
	for rows.Next() {
		var char string
		if err := rows.Scan(&char); err != nil {
			return nil, err
		}
		letters = append(letters, char)
	}

	return letters, nil
}

func GetLetterID(db *sql.DB, letterChar rune) (int, error) {
	var id int
	letterStr := string(letterChar)
	err := db.QueryRow("SELECT id FROM letters WHERE char = ?", letterStr).Scan(&id)
	return id, err
}

func GetAllUsers(db *sql.DB) ([]string, error) {
	rows, err := db.Query("SELECT name FROM users")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		users = append(users, name)
	}
	return users, nil
}

func GetAllLetters(db *sql.DB) ([]string, error) {
	rows, err := db.Query("SELECT char FROM letters")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var letters []string
	for rows.Next() {
		var char string
		if err := rows.Scan(&char); err != nil {
			return nil, err
		}
		letters = append(letters, char)
	}
	return letters, nil
}

func DeleteUser(db *sql.DB, userID int) error {
	_, err := db.Exec("DELETE FROM user_letters WHERE user_id = ?", userID)
	if err != nil {
		return err
	}

	_, err = db.Exec("DELETE FROM users WHERE id = ?", userID)
	return err
}

func UpdateUserName(db *sql.DB, userID int, newName string) error {
	_, err := db.Exec("UPDATE users SET name = ? WHERE id = ?", newName, userID)
	return err
}

func DeleteLetter(db *sql.DB, letterID int) error {
	_, err := db.Exec("DELETE FROM user_letters WHERE letter_id = ?", letterID)
	if err != nil {
		return err
	}

	_, err = db.Exec("DELETE FROM letters WHERE id = ?", letterID)
	return err
}

func EnsureLetterExists(db *sql.DB, letter rune) (int, error) {
	letterStr := string(letter)
	var id int
	err := db.QueryRow("SELECT id FROM letters WHERE char = ?", letterStr).Scan(&id)
	if err != nil {
		return CreateLetter(db, letter)
	}
	return id, nil
}

func UpdateLetter(db *sql.DB, letterID int, newLetter rune) error {
	newLetterStr := string(newLetter)

	// 1. Проверяем, не существует ли УЖЕ буква, в которую мы переименовываем
	var existingID int

	// ИЗМЕНЕНО: было "WHERE letter = ?"
	err := db.QueryRow("SELECT id FROM letters WHERE char = ?", newLetterStr).Scan(&existingID)

	if err == nil {
		// Буква найдена.
		// Если это та же самая буква (тот же ID), то ошибки нет, просто ничего не делаем.
		if existingID == letterID {
			return nil // Переименование в самого себя
		}
		// Если ID другой - значит, такая буква уже занята
		return fmt.Errorf("буква '%s' уже существует в базе данных", newLetterStr)
	}

	// Мы ожидаем ошибку "нет строк", это хорошо
	if err != sql.ErrNoRows {
		// Если ошибка не "нет строк", а какая-то другая - это плохо
		return fmt.Errorf("ошибка при проверке существования буквы: %v", err)
	}

	// 2. Если мы здесь, значит err == sql.ErrNoRows.
	// Этой буквы нет, и мы можем безопасно обновить старую.

	// ИЗМЕНЕНО: было "SET letter = ?"
	_, err = db.Exec("UPDATE letters SET char = ? WHERE id = ?", newLetterStr, letterID)
	if err != nil {
		return fmt.Errorf("ошибка при обновлении буквы: %v", err)
	}

	return nil
}
