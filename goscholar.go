package goscholar

import (
	"bufio"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
)

// ProxyFileURL adalah URL sumber proxy
const ProxyFileURL = "https://raw.githubusercontent.com/TheSpeedX/SOCKS-List/master/http.txt"

// TODO: stuck di proxy

// ProxyList adalah daftar proxy yang akan diperbarui
var ProxyList []string

// DownloadProxyFile mengunduh daftar proxy dari URL
func DownloadProxyFile() error {
	resp, err := http.Get(ProxyFileURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Buat file sementara untuk menyimpan daftar proxy
	file, err := os.CreateTemp("", "proxylist-*.txt")
	if err != nil {
		return err
	}
	defer file.Close()

	// Simpan isi file ke file lokal
	_, err = file.ReadFrom(resp.Body)
	if err != nil {
		return err
	}

	// Parse proxy dari file
	return parseProxyFile(file.Name())
}

// parseProxyFile membaca file lokal dan memperbarui ProxyList
func parseProxyFile(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	var proxies []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			log.Println(line)
			// Tambahkan skema jika tidak ada
			if !strings.HasPrefix(line, "http://") && !strings.HasPrefix(line, "https://") {
				line = "http://" + line
				log.Println(line, "new")
			}
			proxies = append(proxies, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	// Update ProxyList
	ProxyList = proxies
	log.Printf("Updated ProxyList with %d proxies", len(ProxyList))
	return nil
}

func StartProxyUpdater() {
	go func() {
		ticker := time.NewTicker(24 * time.Hour) // Setiap 24 jam
		defer ticker.Stop()

		for {
			<-ticker.C
			err := DownloadProxyFile()
			if err != nil {
				log.Printf("Failed to update proxy list: %v", err)
			} else {
				log.Println("Proxy list updated successfully")
			}
		}
	}()
}

// ScholarData represents the structure for extracted data.
type ScholarData struct {
	Title       string
	Authors     string
	Description string
	Citation    string
}

// CrawlGoogleScholar fetches data based on a search query with rate limiting and proxies.
func CrawlGoogleScholarByUserID(userID string) ([]ScholarData, error) {
	// Pastikan ProxyList tidak kosong
	if len(ProxyList) == 0 {
		err := DownloadProxyFile() // Unduh jika kosong
		if err != nil {
			log.Printf("Failed to fetch proxy list: %v", err)
			return nil, err
		}
	}
	// Create a new collector
	c := colly.NewCollector(
		colly.Async(true), // Enable async scraping
	)

	var results []ScholarData

	// Set a custom transport with timeout
	c.WithTransport(&http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second, // Connection timeout
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout: 10 * time.Second,
	})

	// Set request timeout
	c.SetRequestTimeout(60 * time.Second)
	// Add rate limiting
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*scholar.google.com*",
		Delay:       3 * time.Second, // 3 seconds delay
		RandomDelay: 2 * time.Second, // Add random delay
	})

	// Rotate proxies
	proxyIndex := 0
	c.SetProxyFunc(func(_ *http.Request) (*url.URL, error) {
		if len(ProxyList) == 0 {
			return nil, nil
		}
		proxy := ProxyList[proxyIndex]
		proxyIndex = (proxyIndex + 1) % len(ProxyList)
		return url.Parse(proxy)
	})

	// Debug response status code
	c.OnResponse(func(r *colly.Response) {
		log.Printf("Response status code: %d", r.StatusCode)
	})

	// Callback for scraping the data
	c.OnHTML(".gsc_a_tr", func(e *colly.HTMLElement) {
		data := ScholarData{
			Title:       e.ChildText(".gsc_a_at"),
			Authors:     e.ChildText(".gs_gray"),
			Description: e.ChildText(".gs_rs"),
			Citation:    e.ChildText(".gs_fl a"),
		}
		results = append(results, data)
	})

	// Error handling
	c.OnError(func(_ *colly.Response, err error) {
		log.Printf("Request failed: %v", err)
	})

	// Build the search URL
	searchURL := "https://scholar.google.com/citations?view_op=list_works&hl=id&user=" + url.QueryEscape(userID) + "&pagezise=200"
	log.Println("Visiting:", searchURL)
	err := c.Visit(searchURL)
	if err != nil {
		return nil, err
	}

	// Wait until all asynchronous requests are complete
	c.Wait()

	return results, nil
}
