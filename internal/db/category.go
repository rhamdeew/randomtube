package db

import "database/sql"

type Category struct {
	ID   int64
	Name string
	Code string
}

func ListCategories(db *sql.DB) ([]Category, error) {
	rows, err := db.Query("SELECT id, name, code FROM categories ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cats []Category
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Code); err != nil {
			return nil, err
		}
		cats = append(cats, c)
	}
	return cats, rows.Err()
}

func GetCategoryByCode(db *sql.DB, code string) (*Category, error) {
	var c Category
	err := db.QueryRow("SELECT id, name, code FROM categories WHERE code = ?", code).
		Scan(&c.ID, &c.Name, &c.Code)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &c, err
}

func GetCategoryByID(db *sql.DB, id int64) (*Category, error) {
	var c Category
	err := db.QueryRow("SELECT id, name, code FROM categories WHERE id = ?", id).
		Scan(&c.ID, &c.Name, &c.Code)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &c, err
}

func CreateCategory(db *sql.DB, name, code string) (int64, error) {
	res, err := db.Exec("INSERT INTO categories (name, code) VALUES (?, ?)", name, code)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func UpdateCategory(db *sql.DB, id int64, name, code string) error {
	_, err := db.Exec("UPDATE categories SET name = ?, code = ? WHERE id = ?", name, code, id)
	return err
}

func DeleteCategory(db *sql.DB, id int64) error {
	_, err := db.Exec("DELETE FROM categories WHERE id = ?", id)
	return err
}
