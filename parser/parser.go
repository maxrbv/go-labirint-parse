package parser

import (
	"fmt"
	"net/http"
	"strings"

	"labirint-parser/config"
	"labirint-parser/logger"
	"labirint-parser/models"

	"github.com/gocolly/colly/v2"
)

var collector *colly.Collector

func InitCollector(cfg *config.Config) {
	collector = colly.NewCollector(
		colly.AllowURLRevisit(),
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"),
	)

	collector.OnRequest(func(r *colly.Request) {
		r.Headers.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
		r.Headers.Set("Accept-Language", "ru-RU,ru;q=0.9,en-US;q=0.8,en;q=0.7")
		r.Headers.Set("Cache-Control", "max-age=0")
		r.Headers.Set("Priority", "u=0, i")
		r.Headers.Set("Referer", "https://www.labirint.ru/")
		r.Headers.Set("Sec-Ch-Ua", "\"Chromium\";v=\"124\", \"Google Chrome\";v=\"124\", \"Not-A.Brand\";v=\"99\"")
		r.Headers.Set("Sec-Ch-Ua-Mobile", "?0")
		r.Headers.Set("Sec-Ch-Ua-Platform", "\"Windows\"")
		r.Headers.Set("Sec-Fetch-Dest", "document")
		r.Headers.Set("Sec-Fetch-Mode", "navigate")
		r.Headers.Set("Sec-Fetch-Site", "same-origin")
		r.Headers.Set("Sec-Fetch-User", "?1")
		r.Headers.Set("Upgrade-Insecure-Requests", "1")
	})

	cookies := map[string]string{
		"PHPSESSID":      "0d9kvv5f7jrocq8vt3f50o1dac",
		"id_post":        "2451",
		"UserSes":        "lab0d9kvv5f7jrocq8",
		"br_webp":        "8",
		"tmr_lvid":       "e9a77b749b213444810e4770875ef198",
		"tmr_lvidTS":     "1702769808113",
		"_ym_uid":        "1702769808552808847",
		"_ym_d":          "1714482904",
		"cookie_policy":  "1",
		"begintimed":     "MTcxNDg1NzA3Nw%3D%3D",
		"_ym_isad":       "1",
		"_gid":           "GA1.2.358946072.1714857078",
		"domain_sid":     "Kw3lXRqb9Krjhsy8HLx7X%3A1714857078139",
		"_ym_visorc":     "b",
		"_ga":            "GA1.2.1239653893.1714482903",
		"tmr_detect":     "1%7C1714857297906",
		"_ga_21PJ900698": "GS1.1.1714857077.2.1.1714857301.0.0.0",
	}

	for name, value := range cookies {
		collector.SetCookies(".labirint.ru", []*http.Cookie{{
			Name:   name,
			Value:  value,
			Domain: ".labirint.ru",
			Path:   "/",
		}})
	}

	collector.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: cfg.Parser.Parallel,
		RandomDelay: cfg.Parser.Delay,
	})
}

func ParseBook(url string, logger *logger.Logger, cfg *config.Config) (models.Book, error) {
	var book models.Book
	var parseError error
	book.URL = url
	book.ID = strings.TrimPrefix(url, "https://www.labirint.ru/books/")

	logger.Info("Started parsing book", "url", url)

	c := collector.Clone()

	c.OnError(func(r *colly.Response, err error) {
		parseError = fmt.Errorf("error scraping %s: %v", url, err)
		logger.Error(err, "error parsing book", "url", url)
	})

	c.OnHTML("h1[itemprop=name]", func(e *colly.HTMLElement) {
		book.Title = strings.TrimSpace(e.Text)
	})

	c.OnHTML("div[class^='_prices_']", func(e *colly.HTMLElement) {
		priceDiv := e.DOM.Find("div.rubl")
		if priceDiv.Length() > 0 {
			html, _ := priceDiv.Html()
			book.Price = strings.TrimSpace(html)
		} else {
			priceBaseDiv := e.DOM.Find("div[class^='_priceBase_']")
			if priceBaseDiv.Length() > 0 {
				book.Price = strings.TrimSpace(priceBaseDiv.Text())
			}
		}
	})

	c.OnHTML("div[class^='_block_']", func(e *colly.HTMLElement) {
		availabilityDiv := e.DOM
		if availabilityDiv.Length() > 0 {
			availabilityText := strings.TrimSpace(availabilityDiv.Text())
			if strings.Contains(availabilityText, "Ограниченное количество") {
				book.Availability = "Ограниченное количество"
			} else if strings.Contains(availabilityText, "Нет в продаже") {
				book.Availability = "Нет в продаже"
			} else if strings.Contains(availabilityText, "Ожидается") {
				book.Availability = "Ожидается"
			} else {
				book.Availability = "В наличии"
			}
		} else {
			book.Availability = "Нет в продаже"
		}
	})

	c.OnHTML("div[class^='_gallery_']", func(e *colly.HTMLElement) {
		if !cfg.Parser.ParseImages {
			return
		}
		slideCount := e.DOM.Find("div[class^='_slide_']").Length()
		if slideCount > 0 {
			bookID := book.ID
			imageLinks := []string{
				fmt.Sprintf("https://static10.labirint.ru/books/%s/cover.jpg", bookID),
			}

			for i := 1; i <= slideCount; i++ {
				imageLinks = append(imageLinks, fmt.Sprintf("https://static10.labirint.ru/books/%s/ph_%02d.jpg", bookID, i))
			}

			book.ImageLinks = imageLinks
		}
	})

	err := c.Visit(url)
	if err != nil {
		return models.Book{}, err
	}

	if parseError != nil {
		return models.Book{}, parseError
	}

	return book, nil
}
