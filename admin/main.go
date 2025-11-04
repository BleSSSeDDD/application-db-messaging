package main

import (
	"database/sql"
	"fmt"
	"laba3/database"
	"log"
	"strings"
	"unicode"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/validation"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

type AdminApp struct {
	db           *sql.DB
	window       fyne.Window
	mainTabs     *container.AppTabs
	matrixScroll *container.Scroll
}

func NewAdminApp(db *sql.DB) *AdminApp {
	application := app.New()
	window := application.NewWindow("Администратор системы доступа")
	window.Resize(fyne.NewSize(1200, 800))

	adminApp := &AdminApp{
		db:     db,
		window: window,
	}

	adminApp.mainTabs = container.NewAppTabs(
		container.NewTabItem("Матрица доступа", adminApp.createMatrixTab()),
		container.NewTabItem("Управление пользователями", adminApp.createUserManagementTab()),
	)

	window.SetContent(adminApp.mainTabs)
	return adminApp
}

func (a *AdminApp) ShowAndRun() {
	a.window.ShowAndRun()
}

func (a *AdminApp) createMatrixTab() fyne.CanvasObject {
	refreshBtn := widget.NewButton("Обновить", a.refreshMatrix)

	table := a.createUserTable()
	a.matrixScroll = container.NewScroll(table)

	return container.NewBorder(
		nil,
		container.NewHBox(refreshBtn),
		nil, nil,
		a.matrixScroll,
	)
}

func (a *AdminApp) createUserTable() *widget.Table {
	users, err := database.GetAllUsers(a.db)
	if err != nil {
		log.Printf("Ошибка получения пользователей: %v", err)
		users = []string{}
	}

	letters, err := database.GetAllLetters(a.db)
	if err != nil {
		log.Printf("Ошибка получения букв: %v", err)
		letters = []string{}
	}

	log.Printf("Загружено пользователей: %d, букв: %d", len(users), len(letters))

	table := widget.NewTable(
		func() (int, int) {
			return len(users) + 1, len(letters) + 1
		},
		func() fyne.CanvasObject {
			return container.NewCenter(widget.NewLabel("---"))
		},
		func(id widget.TableCellID, cell fyne.CanvasObject) {
			container := cell.(*fyne.Container)
			label := container.Objects[0].(*widget.Label)

			if id.Row == 0 && id.Col == 0 {
				label.SetText("Пользователь \\ Буква")
				label.Importance = widget.HighImportance
			} else if id.Row == 0 {
				if id.Col-1 < len(letters) {
					label.SetText(letters[id.Col-1])
					label.Importance = widget.HighImportance
				} else {
					label.SetText("")
				}
			} else if id.Col == 0 {
				if id.Row-1 < len(users) {
					label.SetText(users[id.Row-1])
					label.Importance = widget.MediumImportance
				} else {
					label.SetText("")
				}
			} else {
				if id.Row-1 < len(users) && id.Col-1 < len(letters) {
					userName := users[id.Row-1]
					letter := letters[id.Col-1]

					userID, err := database.FindUser(a.db, userName)
					if err != nil {
						label.SetText("❌")
						label.Importance = widget.DangerImportance
						return
					}

					permissions, err := database.GetPermissions(a.db, userID)
					if err != nil {
						label.SetText("❌")
						label.Importance = widget.DangerImportance
						return
					}

					hasAccess := false
					for _, perm := range permissions {
						if perm == letter {
							hasAccess = true
							break
						}
					}

					if hasAccess {
						label.SetText("✓")
						label.Importance = widget.SuccessImportance
					} else {
						label.SetText("✗")
						label.Importance = widget.WarningImportance
					}
				} else {
					label.SetText("")
				}
			}
		},
	)

	table.SetColumnWidth(0, 200)

	table.OnSelected = func(id widget.TableCellID) {
		if id.Row > 0 && id.Col > 0 {
			users, err := database.GetAllUsers(a.db)
			if err != nil {
				dialog.ShowError(err, a.window)
				return
			}

			letters, err := database.GetAllLetters(a.db)
			if err != nil {
				dialog.ShowError(err, a.window)
				return
			}

			if id.Row-1 < len(users) && id.Col-1 < len(letters) {
				userName := users[id.Row-1]
				letter := letters[id.Col-1]
				letterRune := []rune(letter)[0]

				userID, err := database.FindUser(a.db, userName)
				if err != nil {
					dialog.ShowError(err, a.window)
					return
				}

				letterID, err := database.EnsureLetterExists(a.db, letterRune)
				if err != nil {
					dialog.ShowError(err, a.window)
					return
				}

				permissions, err := database.GetPermissions(a.db, userID)
				if err != nil {
					dialog.ShowError(err, a.window)
					return
				}

				hasAccess := false
				for _, perm := range permissions {
					if perm == letter {
						hasAccess = true
						break
					}
				}

				if hasAccess {
					err = database.Remove(a.db, userID, letterID)
					if err != nil {
						dialog.ShowError(err, a.window)
						return
					}
				} else {
					err = database.Grant(a.db, userID, letterID)
					if err != nil {
						dialog.ShowError(err, a.window)
						return
					}
				}

				a.updateMatrixTable()
			}
		}
	}

	return table
}

func (a *AdminApp) updateMatrixTable() {
	if a.matrixScroll != nil {
		newTable := a.createUserTable()
		a.matrixScroll.Content = newTable
		a.matrixScroll.Refresh()
	}
}

func (a *AdminApp) refreshMatrix() {
	a.updateMatrixTable()
}

func validateLetters(input string) error {
	for _, char := range input {
		// Разрешаем пробелы, запятые и точки с запятой в качестве разделителей
		if char != ' ' && char != ',' && char != ';' && !unicode.IsLetter(char) {
			return fmt.Errorf("можно вводить только буквы и разделители (пробел, ',', ';')")
		}
	}
	return nil
}

func validateSingleLetter(input string) error {
	if len([]rune(input)) != 1 {
		return fmt.Errorf("введите ровно один символ")
	}
	char := []rune(input)[0]
	if !unicode.IsLetter(char) {
		return fmt.Errorf("можно вводить только буквы")
	}
	return nil
}

func validateUserList(input string) error {
	lines := strings.Split(input, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue // Пропускаем пустые строки
		}
		if len(trimmed) > 256 {
			return fmt.Errorf("имя '%s' не может быть длиннее 256 символов", trimmed)
		}
	}
	return nil
}

// Вспомогательная функция для парсинга букв из строки
func parseLetters(input string) []rune {
	var letterRunes []rune
	if input != "" {
		for _, char := range input {
			// Игнорируем разделители
			if char != ' ' && char != ',' && char != ';' && char != '\n' && char != '\t' {
				letterRunes = append(letterRunes, char)
			}
		}
	}
	return letterRunes
}

// Вспомогательная функция для парсинга пользователей из строки
func parseUsers(input string) []string {
	var users []string
	lines := strings.Split(input, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			users = append(users, trimmed)
		}
	}
	return users
}

func (a *AdminApp) createUserManagementTab() fyne.CanvasObject {

	// --- Управление одним пользователем (существующий функционал) ---
	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("Введите имя пользователя")
	nameEntry.Validator = validation.NewAllStrings(func(s string) error {
		if len(s) > 256 {
			return fmt.Errorf("имя не может быть длиннее 256 символов")
		}
		if len(s) == 0 {
			return fmt.Errorf("имя не может быть пустым")
		}
		return nil
	})

	lettersEntry := widget.NewEntry()
	lettersEntry.SetPlaceHolder("Введите буквы через пробел (например: A B C или А Б В)\nМожно использовать русские и латинские буквы")
	lettersEntry.MultiLine = true
	lettersEntry.Wrapping = fyne.TextWrapWord
	lettersEntry.Validator = validation.NewAllStrings(validateLetters)

	addUserBtn := widget.NewButton("Добавить пользователя", func() {
		if nameEntry.Validate() != nil {
			dialog.ShowError(fmt.Errorf("неверное имя пользователя"), a.window)
			return
		}

		if lettersEntry.Validate() != nil {
			dialog.ShowError(fmt.Errorf("в поле букв можно вводить только буквы и пробелы"), a.window)
			return
		}

		letterRunes := parseLetters(lettersEntry.Text)

		err := database.Create(a.db, nameEntry.Text, letterRunes...)
		if err != nil {
			dialog.ShowError(err, a.window)
			return
		}

		dialog.ShowInformation("Успех", "Пользователь добавлен", a.window)
		nameEntry.SetText("")
		lettersEntry.SetText("")
		a.refreshAllTabs()
	})

	// --- Массовое управление пользователями (НОВЫЙ/ИЗМЕНЕННЫЙ ФУНКЦИОНАЛ) ---
	bulkUserEntry := widget.NewEntry()
	bulkUserEntry.SetPlaceHolder("Введите имена пользователей, каждое с новой строки")
	bulkUserEntry.MultiLine = true
	bulkUserEntry.Wrapping = fyne.TextWrapWord
	bulkUserEntry.Validator = validation.NewAllStrings(validateUserList)

	// НОВОЕ ПОЛЕ для букв
	bulkLettersEntry := widget.NewEntry()
	bulkLettersEntry.SetPlaceHolder("Введите буквы через пробел (например: A B C)")
	bulkLettersEntry.Validator = validation.NewAllStrings(validateLetters)

	// ИЗМЕНЕННАЯ КНОПКА (Добавить/Выдать права)
	bulkGrantAddBtn := widget.NewButton("Массово выдать права / Добавить", func() {
		if bulkUserEntry.Validate() != nil {
			dialog.ShowError(fmt.Errorf("неверный формат списка пользователей: %v", bulkUserEntry.Validate()), a.window)
			return
		}
		if bulkLettersEntry.Validate() != nil {
			dialog.ShowError(fmt.Errorf("неверный формат списка букв: %v", bulkLettersEntry.Validate()), a.window)
			return
		}

		users := parseUsers(bulkUserEntry.Text)
		letterRunes := parseLetters(bulkLettersEntry.Text)

		if len(users) == 0 {
			dialog.ShowInformation("Внимание", "Список пользователей пуст", a.window)
			return
		}
		if len(letterRunes) == 0 {
			dialog.ShowInformation("Внимание", "Список букв пуст. Права не будут выданы.", a.window)
			// (Можно и продолжить, просто создав пользователей, но лучше уведомить)
		}

		var createdCount, grantedCount int
		var errorMessages []string

		for _, user := range users {
			userID, err := database.FindUser(a.db, user)

			if err != nil {
				// Пользователь не найден -> Создаем
				errCreate := database.Create(a.db, user, letterRunes...)
				if errCreate != nil {
					errorMessages = append(errorMessages, fmt.Sprintf("Ошибка создания '%s': %v", user, errCreate))
				} else {
					createdCount++
				}
			} else {
				// Пользователь найден -> Выдаем права
				var grantedForUser bool
				for _, r := range letterRunes {
					letterID, errEnsure := database.EnsureLetterExists(a.db, r)
					if errEnsure != nil {
						errorMessages = append(errorMessages, fmt.Sprintf("Ошибка (EnsureLetter) для '%s': %v", user, errEnsure))
						continue
					}
					errGrant := database.Grant(a.db, userID, letterID)
					if errGrant != nil {
						// (database.Grant может возвращать ошибку, если право уже есть,
						// в зависимости от реализации. Предполагаем, что он идемпотентен или игнорирует дубликаты)
						// log.Printf("Ошибка выдачи права %c для %s: %v", r, user, errGrant)
					} else {
						grantedForUser = true
					}
				}
				if grantedForUser {
					grantedCount++ // Считаем, что права выданы хотя бы одному
				}
			}
		}

		msg := fmt.Sprintf("Операция завершена.\nСоздано новых пользователей: %d\nВыданы права (существующим): %d", createdCount, grantedCount)
		if len(errorMessages) > 0 {
			dialog.ShowError(fmt.Errorf("%s\n\nОшибки: \n%s", msg, strings.Join(errorMessages, "\n")), a.window)
		} else {
			dialog.ShowInformation("Успех", msg, a.window)
		}
		a.refreshAllTabs()
	})

	// НОВАЯ КНОПКА (Забрать права)
	bulkRemoveRightsBtn := widget.NewButton("Массово забрать права", func() {
		if bulkUserEntry.Validate() != nil {
			dialog.ShowError(fmt.Errorf("неверный формат списка пользователей: %v", bulkUserEntry.Validate()), a.window)
			return
		}
		if bulkLettersEntry.Validate() != nil {
			dialog.ShowError(fmt.Errorf("неверный формат списка букв: %v", bulkLettersEntry.Validate()), a.window)
			return
		}

		users := parseUsers(bulkUserEntry.Text)
		letterRunes := parseLetters(bulkLettersEntry.Text)

		if len(users) == 0 {
			dialog.ShowInformation("Внимание", "Список пользователей пуст", a.window)
			return
		}
		if len(letterRunes) == 0 {
			dialog.ShowInformation("Внимание", "Список букв пуст. Права не будут забраны.", a.window)
			return
		}

		var removedCount int
		var errorMessages []string

		for _, user := range users {
			userID, err := database.FindUser(a.db, user)
			if err != nil {
				errorMessages = append(errorMessages, fmt.Sprintf("Ошибка: пользователь '%s' не найден.", user))
				continue
			}

			var removedForUser bool
			for _, r := range letterRunes {
				letterID, errGetID := database.GetLetterID(a.db, r)
				if errGetID != nil {
					// Если буквы нет в БД, право на нее и так ни у кого нет, это не ошибка
					continue
				}

				errRemove := database.Remove(a.db, userID, letterID)
				if errRemove != nil {
					// log.Printf("Ошибка удаления права %c у %s: %v", r, user, errRemove)
				} else {
					removedForUser = true
				}
			}
			if removedForUser {
				removedCount++
			}
		}

		msg := fmt.Sprintf("Операция завершена.\nОбработано пользователей (у кого забраны права): %d", removedCount)
		if len(errorMessages) > 0 {
			dialog.ShowError(fmt.Errorf("%s\n\nОшибки: \n%s", msg, strings.Join(errorMessages, "\n")), a.window)
		} else {
			dialog.ShowInformation("Успех", msg, a.window)
		}
		a.refreshAllTabs()
	})

	// Старая кнопка (Удалить пользователей)
	bulkDeleteBtn := widget.NewButton("Массово удалить пользователей (Опасно!)", func() {
		if bulkUserEntry.Validate() != nil {
			dialog.ShowError(fmt.Errorf("неверный формат списка пользователей: %v", bulkUserEntry.Validate()), a.window)
			return
		}

		usersToDelete := parseUsers(bulkUserEntry.Text)

		if len(usersToDelete) == 0 {
			dialog.ShowInformation("Внимание", "Список пользователей для удаления пуст", a.window)
			return
		}

		confirm := dialog.NewConfirm("Подтверждение массового удаления",
			fmt.Sprintf("Вы уверены, что хотите УДАЛИТЬ следующих пользователей (%d):\n%s", len(usersToDelete), strings.Join(usersToDelete, ", ")),
			func(confirmed bool) {
				if confirmed {
					var successCount int
					var errorMessages []string

					for _, user := range usersToDelete {
						userID, err := database.FindUser(a.db, user)
						if err != nil {
							errorMessages = append(errorMessages, fmt.Sprintf("Ошибка поиска ID для '%s': %v", user, err))
							continue
						}

						err = database.DeleteUser(a.db, userID)
						if err != nil {
							errorMessages = append(errorMessages, fmt.Sprintf("Ошибка удаления '%s': %v", user, err))
						} else {
							successCount++
						}
					}

					if successCount > 0 {
						if len(errorMessages) > 0 {
							dialog.ShowError(fmt.Errorf("удалено пользователей: %d. Ошибки: \n%s", successCount, strings.Join(errorMessages, "\n")), a.window)
						} else {
							dialog.ShowInformation("Успех", fmt.Sprintf("Массовое удаление завершено. Удалено пользователей: %d", successCount), a.window)
						}
						bulkUserEntry.SetText("")
						bulkLettersEntry.SetText("")
						a.refreshAllTabs()
					} else {
						dialog.ShowError(fmt.Errorf("не удалось удалить ни одного пользователя. Ошибки: \n%s", strings.Join(errorMessages, "\n")), a.window)
					}
				}
			}, a.window)
		confirm.Show()
	})

	// --- Существующий функционал ниже ---
	userSelect := widget.NewSelect([]string{}, nil)
	letterSelect := widget.NewSelect([]string{}, nil)

	updateUserList := func() {
		users, err := database.GetAllUsers(a.db)
		if err != nil {
			log.Printf("Ошибка обновления списка пользователей: %v", err)
			users = []string{}
		}
		userSelect.Options = users
		userSelect.Refresh()
	}

	updateLetterList := func() {
		letters, err := database.GetAllLetters(a.db)
		if err != nil {
			log.Printf("Ошибка обновления списка букв: %v", err)
			letters = []string{}
		}
		letterSelect.Options = letters
		letterSelect.Refresh()
	}

	updateUserList()
	updateLetterList()

	grantAllBtn := widget.NewButton("Выдать все права", func() {
		if userSelect.Selected == "" {
			dialog.ShowInformation("Внимание", "Выберите пользователя", a.window)
			return
		}
		userID, err := database.FindUser(a.db, userSelect.Selected)
		if err != nil {
			dialog.ShowError(err, a.window)
			return
		}
		err = database.GrantAll(a.db, userID)
		if err != nil {
			dialog.ShowError(err, a.window)
			return
		}
		dialog.ShowInformation("Успех", "Все права выданы", a.window)
		a.refreshAllTabs()
	})

	removeAllBtn := widget.NewButton("Забрать все права", func() {
		if userSelect.Selected == "" {
			dialog.ShowInformation("Внимание", "Выберите пользователя", a.window)
			return
		}
		userID, err := database.FindUser(a.db, userSelect.Selected)
		if err != nil {
			dialog.ShowError(err, a.window)
			return
		}
		err = database.RemoveAll(a.db, userID)
		if err != nil {
			dialog.ShowError(err, a.window)
			return
		}
		dialog.ShowInformation("Успех", "Все права забраны", a.window)
		a.refreshAllTabs()
	})

	deleteUserBtn := widget.NewButton("Удалить пользователя", func() {
		if userSelect.Selected == "" {
			dialog.ShowInformation("Внимание", "Выберите пользователя", a.window)
			return
		}
		confirm := dialog.NewConfirm("Подтверждение",
			fmt.Sprintf("Вы уверены, что хотите удалить пользователя %s?", userSelect.Selected),
			func(confirmed bool) {
				if confirmed {
					userID, err := database.FindUser(a.db, userSelect.Selected)
					if err != nil {
						dialog.ShowError(err, a.window)
						return
					}
					err = database.DeleteUser(a.db, userID)
					if err != nil {
						dialog.ShowError(err, a.window)
						return
					}
					dialog.ShowInformation("Успех", "Пользователь удален", a.window)
					updateUserList()
					a.refreshAllTabs()
				}
			}, a.window)
		confirm.Show()
	})

	editUserBtn := widget.NewButton("Редактировать пользователя", func() {
		if userSelect.Selected == "" {
			dialog.ShowInformation("Внимание", "Выберите пользователя", a.window)
			return
		}
		newNameEntry := widget.NewEntry()
		newNameEntry.SetText(userSelect.Selected)
		newNameEntry.Validator = validation.NewAllStrings(func(s string) error {
			if len(s) > 256 {
				return fmt.Errorf("имя не может быть длиннее 256 символов")
			}
			if len(s) == 0 {
				return fmt.Errorf("имя не может быть пустым")
			}
			return nil
		})
		form := dialog.NewForm(
			"Редактирование пользователя",
			"Сохранить",
			"Отмена",
			[]*widget.FormItem{
				{Text: "Новое имя", Widget: newNameEntry},
			},
			func(confirmed bool) {
				if confirmed && newNameEntry.Validate() == nil {
					userID, err := database.FindUser(a.db, userSelect.Selected)
					if err != nil {
						dialog.ShowError(err, a.window)
						return
					}
					err = database.UpdateUserName(a.db, userID, newNameEntry.Text)
					if err != nil {
						dialog.ShowError(err, a.window)
						return
					}
					dialog.ShowInformation("Успех", "Имя пользователя изменено", a.window)
					updateUserList()
					a.refreshAllTabs()
				}
			},
			a.window,
		)
		form.Show()
	})

	deleteLetterBtn := widget.NewButton("Удалить букву", func() {
		if letterSelect.Selected == "" {
			dialog.ShowInformation("Внимание", "Выберите букву", a.window)
			return
		}
		confirm := dialog.NewConfirm("Подтверждение",
			fmt.Sprintf("Вы уверены, что хотите удалить букву %s?\nЭто удалит все права доступа к этой букве у всех пользователей.", letterSelect.Selected),
			func(confirmed bool) {
				if confirmed {
					letterRune := []rune(letterSelect.Selected)[0]
					letterID, err := database.GetLetterID(a.db, letterRune)
					if err != nil {
						dialog.ShowError(err, a.window)
						return
					}
					err = database.DeleteLetter(a.db, letterID)
					if err != nil {
						dialog.ShowError(err, a.window)
						return
					}
					dialog.ShowInformation("Успех", "Буква удалена", a.window)
					updateLetterList()
					a.refreshAllTabs()
				}
			}, a.window)
		confirm.Show()
	})

	addLetterEntry := widget.NewEntry()
	addLetterEntry.SetPlaceHolder("Введите букву для добавления (русскую или латинскую)")
	addLetterEntry.Validator = validation.NewAllStrings(validateSingleLetter)

	addLetterBtn := widget.NewButton("Добавить букву", func() {
		if addLetterEntry.Validate() != nil {
			dialog.ShowError(fmt.Errorf("введите одну букву (русскую или латинскую)"), a.window)
			return
		}
		letterRune := []rune(addLetterEntry.Text)[0]
		_, err := database.EnsureLetterExists(a.db, letterRune)
		if err != nil {
			dialog.ShowError(err, a.window)
			return
		}
		dialog.ShowInformation("Успех", "Буква добавлена", a.window)
		addLetterEntry.SetText("")
		updateLetterList()
		a.refreshAllTabs()
	})

	refreshBtn := widget.NewButton("Обновить списки", func() {
		updateUserList()
		updateLetterList()
		a.refreshAllTabs()
	})

	addSingleUserForm := container.NewVBox(
		widget.NewLabelWithStyle("Добавить одного пользователя", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel("Имя пользователя:"),
		nameEntry,
		widget.NewLabel("Буквы доступа (через пробел):"),
		container.NewBorder(nil, nil, nil, nil, lettersEntry),
		addUserBtn,
	)

	bulkUserForm := container.NewVBox(
		widget.NewLabelWithStyle("Массовое управление пользователями", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel("Список имен (каждое с новой строки):"),
		container.NewBorder(nil, nil, nil, nil, bulkUserEntry),
		widget.NewLabel("Список букв (через пробел):"),
		bulkLettersEntry,
		container.NewHBox(bulkGrantAddBtn, bulkRemoveRightsBtn),
		widget.NewSeparator(),
		bulkDeleteBtn,
	)

	addForm := container.NewVBox(
		addSingleUserForm,
		widget.NewSeparator(),
		bulkUserForm,
		widget.NewSeparator(),
	)

	userManageForm := container.NewVBox(
		widget.NewLabelWithStyle("Управление одним пользователем", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		userSelect,
		container.NewHBox(grantAllBtn, removeAllBtn),
		container.NewHBox(editUserBtn, deleteUserBtn),
		widget.NewSeparator(),
	)

	letterManageForm := container.NewVBox(
		widget.NewLabelWithStyle("Управление буквами", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel("Добавить одну букву:"),
		container.NewVBox(
			addLetterEntry,
			addLetterBtn,
		),
		widget.NewLabel("Удалить букву:"),
		letterSelect,
		deleteLetterBtn,
		widget.NewSeparator(),
	)

	content := container.NewVBox(
		addForm,
		container.NewGridWithColumns(2,
			userManageForm,
			letterManageForm,
		),
		refreshBtn,
	)

	return container.NewScroll(content)
}

func (a *AdminApp) refreshAllTabs() {
	a.updateMatrixTable()
	userManagementTab := a.createUserManagementTab()
	a.mainTabs.Items[1].Content = userManagementTab
	a.mainTabs.Refresh()
}

func main() {
	db, err := database.Init("data.db")
	if err != nil {
		log.Fatal("Ошибка инициализации базы данных:", err)
	}
	defer db.Close()

	app := NewAdminApp(db)
	app.ShowAndRun()
}
