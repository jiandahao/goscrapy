package cache

import (
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"net/url"

	"os"
	"path/filepath"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/jiandahao/goscrapy"
)

// DiskCache cache response into disk.
type DiskCache struct {
	storageDir string
}

// NewDiskCache new downloader cache
func NewDiskCache(dir string) *DiskCache {
	return &DiskCache{
		storageDir: strings.TrimSuffix(dir, "/"),
	}
}

// Store store request
func (c *DiskCache) Store(resp *goscrapy.Response) (err error) {
	html, err := resp.Document.Html()
	if err != nil {
		return err
	}

	filePath, err := c.formatFilePath(resp.Request.URL)
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return err
	}

	fd, err := os.OpenFile(filePath, os.O_CREATE|os.O_RDWR, os.ModePerm)
	if err != nil {
		return err
	}
	defer fd.Close()

	_, err = fd.Write([]byte(html))
	if err != nil {
		return err
	}

	return nil
}

// Load load response from cache
func (c *DiskCache) Load(req *goscrapy.Request) (*goscrapy.Response, error) {
	filePath, err := c.formatFilePath(req.URL)
	if err != nil {
		return nil, err
	}

	fd, err := os.Open(filePath)
	if os.IsNotExist(err) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	defer fd.Close()

	doc, err := goquery.NewDocumentFromReader(fd)
	if err != nil {
		return nil, err
	}

	return &goscrapy.Response{
		Request:    req,
		Document:   doc,
		Status:     "OK (from disk cache)",
		StatusCode: 200,
	}, nil
}

func (c *DiskCache) formatFilePath(urlStr string) (string, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return "", err
	}

	hash := md5.New()
	hash.Write([]byte(urlStr))
	fileHash := base64.StdEncoding.EncodeToString(hash.Sum(nil))

	path := filepath.Join(c.storageDir, u.Host, u.Path, fmt.Sprintf("%s.html", fileHash))
	return path, nil
}
