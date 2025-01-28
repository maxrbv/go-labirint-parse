package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"labirint-parser/config"
	"labirint-parser/logger"
	"labirint-parser/models"
	"labirint-parser/parser"

	"github.com/xuri/excelize/v2"
)

func main() {
	startTime := time.Now()

	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	logger := logger.NewLogger(cfg.Logger)

	bookIDs, err := config.LoadUrls(cfg.Parser.BooksIdsFile)
	if err != nil {
		logger.Error(err, "Error loading urls")
		return
	}
	logger.Info("Loaded books IDs", "count", len(bookIDs))

	parser.InitCollector(&cfg)

	maxConcurrent := cfg.Parser.Parallel
	sem := make(chan struct{}, maxConcurrent)
	results := make(chan models.Book, maxConcurrent)
	errors := make(chan error, maxConcurrent)

	var wg sync.WaitGroup
	var collectorWg sync.WaitGroup
	var books []models.Book
	var errs []error
	var mu sync.Mutex

	collectorWg.Add(1)
	go func() {
		defer collectorWg.Done()
		for {
			select {
			case book, ok := <-results:
				if !ok {
					return
				}
				mu.Lock()
				books = append(books, book)
				booksLen := len(books)
				mu.Unlock()

				logger.Info("Book parsed successfully",
					"id", book.ID,
					"total_parsed", booksLen,
				)
			case err, ok := <-errors:
				if !ok {
					return
				}
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
				logger.Error(err, "Failed to parse book")
			}
		}
	}()

	for i, bookID := range bookIDs {
		wg.Add(1)
		sem <- struct{}{}

		go func(id string, index int) {
			defer wg.Done()
			defer func() { <-sem }()

			bookURL := fmt.Sprintf("https://www.labirint.ru/books/%s", id)
			logger.Info("Starting to parse book",
				"url", bookURL,
				"progress", fmt.Sprintf("%d/%d", index+1, len(bookIDs)),
			)

			book, err := parser.ParseBook(bookURL, logger, &cfg)
			if err != nil {
				errors <- fmt.Errorf("error parsing book %s: %v", id, err)
				return
			}

			results <- book
		}(bookID, i)
	}

	wg.Wait()
	close(results)
	close(errors)
	collectorWg.Wait()

	duration := time.Since(startTime)
	logger.Info("Parsing completed",
		"total_books", len(bookIDs),
		"successful", len(books),
		"failed", len(errs),
		"duration", duration.String(),
	)

	if len(books) > 0 {
		if resultsName, err := saveResults(books, cfg); err != nil {
			logger.Error(err, "Failed to save results")
		} else {
			logger.Info("Results saved successfully", "file", resultsName)
		}

		if excelName, err := saveExcel(books, cfg); err != nil {
			logger.Error(err, "Failed to save Excel results")
		} else {
			logger.Info("Excel results saved successfully", "file", excelName)
		}
	}
}

func saveResults(books []models.Book, cfg config.Config) (string, error) {
	file, err := json.MarshalIndent(books, "", "  ")
	if err != nil {
		return "", fmt.Errorf("error marshaling results: %v", err)
	}

	currentTime := time.Now()
	dateStr := currentTime.Format("2006-01-02")
	ext := filepath.Ext(cfg.Parser.OutputFile)
	base := strings.TrimSuffix(cfg.Parser.OutputFile, ext)
	resultsName := fmt.Sprintf("%s_%s%s", base, dateStr, ext)

	err = os.WriteFile(resultsName, file, 0644)
	if err != nil {
		return "", fmt.Errorf("error writing results file: %v", err)
	}
	return resultsName, nil
}

func saveExcel(books []models.Book, cfg config.Config) (string, error) {
	f := excelize.NewFile()
	defer f.Close()

	headers := []string{"Ссылка", "Название", "ID", "Наличие", "Цена"}
	style, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold: true,
		},
	})

	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue("Sheet1", cell, header)
		f.SetCellStyle("Sheet1", cell, cell, style)
	}

	maxImageLinks := 0
	for _, book := range books {
		if len(book.ImageLinks) > maxImageLinks {
			maxImageLinks = len(book.ImageLinks)
		}
	}

	for i := 0; i < maxImageLinks; i++ {
		cell, _ := excelize.CoordinatesToCellName(len(headers)+i+1, 1)
		f.SetCellValue("Sheet1", cell, fmt.Sprintf("Картинка_%d", i+1))
		f.SetCellStyle("Sheet1", cell, cell, style)
	}

	for i, book := range books {
		row := i + 2
		f.SetCellValue("Sheet1", fmt.Sprintf("A%d", row), book.URL)
		f.SetCellValue("Sheet1", fmt.Sprintf("B%d", row), book.Title)
		f.SetCellValue("Sheet1", fmt.Sprintf("C%d", row), book.ID)
		f.SetCellValue("Sheet1", fmt.Sprintf("D%d", row), book.Availability)
		f.SetCellValue("Sheet1", fmt.Sprintf("E%d", row), book.Price)

		for j, link := range book.ImageLinks {
			cell, _ := excelize.CoordinatesToCellName(len(headers)+j+1, row)
			f.SetCellValue("Sheet1", cell, link)
		}
	}

	currentTime := time.Now()
	dateStr := currentTime.Format("2006-01-02")
	excelName := strings.TrimSuffix(cfg.Parser.OutputFile, filepath.Ext(cfg.Parser.OutputFile)) + "_" + dateStr + ".xlsx"

	if err := f.SaveAs(excelName); err != nil {
		return "", fmt.Errorf("error saving excel file: %v", err)
	}

	return excelName, nil
}
