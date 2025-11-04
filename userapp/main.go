package main

import (
	"database/sql"
	"fmt"
	"laba3/database"
	"log"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type TextProcessor struct {
	db           *sql.DB
	mainWindow   fyne.Window
	currentUser  int
	username     string
	accessRights map[rune]bool
	autoRefresh  *time.Timer
}

func CreateTextProcessor(db *sql.DB) *TextProcessor {
	application := app.New()
	application.Settings().SetTheme(theme.DarkTheme())

	window := application.NewWindow("Текст Процессор")
	window.Resize(fyne.NewSize(800, 600))

	textProcessor := &TextProcessor{
		db:           db,
		mainWindow:   window,
		accessRights: make(map[rune]bool),
	}

	textProcessor.displayAuthScreen()

	return textProcessor
}

func (tp *TextProcessor) Launch() {
	tp.mainWindow.ShowAndRun()
}

func (tp *TextProcessor) displayAuthScreen() {
	usernameInput := widget.NewEntry()
	usernameInput.SetPlaceHolder("Ваше имя пользователя")
	usernameInput.OnSubmitted = func(text string) {
		tp.authenticateUser(text)
	}

	authButton := widget.NewButton("Подтвердить вход", func() {
		tp.authenticateUser(usernameInput.Text)
	})
	authButton.Importance = widget.HighImportance

	welcomeText := widget.NewRichTextWithText("Текст Процессор")
	welcomeText.Segments[0].(*widget.TextSegment).Style = widget.RichTextStyle{
		TextStyle: fyne.TextStyle{Bold: true},
	}

	instructionText := widget.NewLabel("Для начала работы введите ваше имя пользователя")

	formContainer := container.NewVBox(
		welcomeText,
		instructionText,
		usernameInput,
		authButton,
	)

	centeredContent := container.NewCenter(formContainer)
	tp.mainWindow.SetContent(centeredContent)
}

func (tp *TextProcessor) authenticateUser(name string) {
	if strings.TrimSpace(name) == "" {
		dialog.ShowError(fmt.Errorf("необходимо указать имя пользователя"), tp.mainWindow)
		return
	}

	userID, err := database.FindUser(tp.db, name)
	if err != nil {
		if err == sql.ErrNoRows {
			dialog.ShowError(fmt.Errorf("пользователь '%s' не зарегистрирован", name), tp.mainWindow)
		} else {
			dialog.ShowError(fmt.Errorf("ошибка подключения к базе: %v", err), tp.mainWindow)
		}
		return
	}

	tp.currentUser = userID
	tp.username = name

	tp.loadAccessRights()

	log.Printf("Сессия пользователя %s активна. Доступные символы: %v", name, tp.getAccessList())

	tp.displayWorkArea()

	tp.initAutoRefresh()
}

func (tp *TextProcessor) loadAccessRights() {
	rights, err := database.GetPermissions(tp.db, tp.currentUser)
	if err != nil {
		log.Printf("Ошибка загрузки прав доступа: %v", err)
		return
	}

	tp.accessRights = make(map[rune]bool)
	for _, right := range rights {
		if len(right) > 0 {
			tp.accessRights[[]rune(right)[0]] = true
		}
	}
}

func (tp *TextProcessor) getAccessList() []string {
	allowed := make([]string, 0, len(tp.accessRights))
	for char := range tp.accessRights {
		allowed = append(allowed, string(char))
	}
	return allowed
}

func (tp *TextProcessor) initAutoRefresh() {
	if tp.autoRefresh != nil {
		tp.autoRefresh.Stop()
	}

	tp.autoRefresh = time.AfterFunc(2*time.Second, func() {
		previousRights := make(map[rune]bool)
		for k, v := range tp.accessRights {
			previousRights[k] = v
		}

		tp.loadAccessRights()

		modified := len(tp.accessRights) != len(previousRights)
		if !modified {
			for char := range tp.accessRights {
				if !previousRights[char] {
					modified = true
					break
				}
			}
		}

		if modified {
			log.Printf("Обновлены права доступа для %s: %v", tp.username, tp.getAccessList())
			tp.updateInterface()
		}

		tp.initAutoRefresh()
	})
}

func (tp *TextProcessor) updateInterface() {
	if tp.mainWindow.Content() != nil {
		tp.displayWorkArea()
	}
}

func (tp *TextProcessor) displayWorkArea() {
	textInput := widget.NewMultiLineEntry()
	textInput.SetPlaceHolder("Введите ваш текст здесь для обработки...")
	textInput.Wrapping = fyne.TextWrapWord

	resultsDisplay := widget.NewLabel("")
	resultsDisplay.Wrapping = fyne.TextWrapWord
	resultsDisplay.TextStyle = fyne.TextStyle{Bold: true}

	processAction := widget.NewButton("Выполнить фильтрацию", func() {
		inputText := textInput.Text
		if strings.TrimSpace(inputText) == "" {
			dialog.ShowInformation("Информация", "Пожалуйста, введите текст для обработки", tp.mainWindow)
			return
		}

		processedText := tp.applyFilter(inputText)
		resultsDisplay.SetText(processedText)
	})
	processAction.Importance = widget.HighImportance

	resetAction := widget.NewButton("Сбросить все", func() {
		textInput.SetText("")
		resultsDisplay.SetText("")
	})

	reloadRights := widget.NewButton("Перезагрузить права", func() {
		tp.loadAccessRights()
		tp.updateInterface()
		dialog.ShowInformation("Готово", "Права доступа успешно перезагружены", tp.mainWindow)
	})

	endSession := widget.NewButton("Завершить сеанс", func() {
		if tp.autoRefresh != nil {
			tp.autoRefresh.Stop()
		}
		tp.currentUser = 0
		tp.username = ""
		tp.accessRights = make(map[rune]bool)
		tp.displayAuthScreen()
	})

	userProfile := widget.NewLabel(fmt.Sprintf("Текущий пользователь: %s", tp.username))

	rightsInfo := widget.NewLabel(fmt.Sprintf("Разрешенные символы: %s", strings.Join(tp.getAccessList(), ", ")))

	autoRefreshStatus := widget.NewLabel("Автообновление прав: активно (интервал 2 сек)")

	headerSection := container.NewVBox(
		userProfile,
		rightsInfo,
		autoRefreshStatus,
		widget.NewSeparator(),
	)

	inputSection := container.NewVBox(
		widget.NewLabel("Исходный текст:"),
		textInput,
	)

	controlPanel := container.NewHBox(
		processAction,
		resetAction,
		reloadRights,
	)

	outputSection := container.NewVBox(
		widget.NewLabel("Обработанный текст:"),
		resultsDisplay,
	)

	footerSection := container.NewVBox(
		widget.NewSeparator(),
		endSession,
	)

	mainContent := container.NewVBox(
		headerSection,
		inputSection,
		controlPanel,
		outputSection,
		footerSection,
	)

	scrollableContent := container.NewScroll(mainContent)
	tp.mainWindow.SetContent(scrollableContent)
}

func (tp *TextProcessor) applyFilter(input string) string {
	var filtered strings.Builder

	for _, char := range input {
		if char == ' ' || char == '\n' || char == '\t' || char == '\r' {
			filtered.WriteRune(char)
			continue
		}

		if tp.accessRights[char] {
			filtered.WriteRune(char)
		}
	}

	return filtered.String()
}

func (tp *TextProcessor) Shutdown() {
	if tp.autoRefresh != nil {
		tp.autoRefresh.Stop()
	}
}

func main() {
	db, err := database.Init("data.db")
	if err != nil {
		log.Fatal("Ошибка инициализации базы данных:", err)
	}
	defer db.Close()

	textProcessor := CreateTextProcessor(db)

	defer textProcessor.Shutdown()

	textProcessor.Launch()
}

