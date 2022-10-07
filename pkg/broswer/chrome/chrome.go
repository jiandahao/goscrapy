package chrome

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/tebeka/selenium"
	"github.com/tebeka/selenium/chrome"
)

const (
	stateClosed  int32 = 0
	stateOpenned int32 = 1
)

// Broswer chrome broswer instance
type Broswer struct {
	selenium.WebDriver
	service  *selenium.Service
	cfg      Config
	stopOnce sync.Once
	state    int32
}

// Config chrome config
type Config struct {
	ChromeDriverPath  string
	Port              int
	Headless          bool
	UserAgent         string
	DownloadDirectory string // default download directory
	ProxyURL          string // proxy url
}

// NewBroswer new a chrome broswer
func NewBroswer(cfg Config) (*Broswer, error) {
	if cfg.ChromeDriverPath == "" {
		cfg.ChromeDriverPath = "/usr/bin/chromedriver"
	}

	return &Broswer{
		cfg:   cfg,
		state: stateClosed,
	}, nil
}

// Open open broswer
func (b *Broswer) Open() error {
	if atomic.LoadInt32(&b.state) == stateOpenned {
		return nil
	}
	var err error

	defer func() {
		if err == nil {
			atomic.StoreInt32(&b.state, stateOpenned)
		}
	}()

	cfg := b.cfg

	// start selenium service
	ops := []selenium.ServiceOption{}
	b.service, err = selenium.NewChromeDriverService(cfg.ChromeDriverPath, cfg.Port, ops...)
	if err != nil {
		return fmt.Errorf("Error starting the ChromeDriver server: %v", err)
	}

	//2. setting chrome as our broswer
	caps := selenium.Capabilities{
		"browserName": "chrome",
	}

	chrCaps := chrome.Capabilities{
		Prefs: map[string]interface{}{},
		Args:  []string{},
	}

	if cfg.UserAgent != "" {
		chrCaps.Args = append(chrCaps.Args, fmt.Sprintf("--user-agent=%s", cfg.UserAgent))
	}

	if cfg.ProxyURL != "" {
		chrCaps.Args = append(chrCaps.Args, fmt.Sprintf("--proxy-server=%s", cfg.ProxyURL))
	}

	if cfg.Headless {
		chrCaps.Prefs["profile.managed_default_content_settings.images"] = 2
		chrCaps.Args = append(chrCaps.Args,
			"--headless",   // setting headless mode
			"--no-sandbox", // using Chrome without sandbox
			"--disable-dev-shm-usage",
			"--disable-gpu",
			"–disable-setuid-sandbox",
			"–-no-first-run",
			"–-single-process",
			"--disable-extensions",
		)
	}

	if cfg.DownloadDirectory != "" {
		chrCaps.Prefs["download.default_directory"] = cfg.DownloadDirectory // setting default download directory
	}

	caps.AddChrome(chrCaps)

	fmt.Println("Creating new remote client to selenium server: ", fmt.Sprintf("http://127.0.0.1:%v/wd/hub", cfg.Port))
	b.WebDriver, err = selenium.NewRemote(caps, fmt.Sprintf("http://127.0.0.1:%v/wd/hub", cfg.Port))
	if err != nil {
		return err
	}

	return nil
}

// IsOpenned returns true if broswer has been openned.
func (b *Broswer) IsOpenned() bool {
	return atomic.LoadInt32(&b.state) == stateOpenned
}

// Close close broswer
func (b *Broswer) Close() error {
	if atomic.LoadInt32(&b.state) == stateClosed {
		return nil
	}

	var err error
	if b.WebDriver != nil {
		err = b.WebDriver.Quit()
	}

	if b.service != nil {
		err = b.service.Stop()
	}

	if err == nil {
		atomic.StoreInt32(&b.state, stateClosed)
	}

	return err
}

// DownloadDefaultDirectory returns default download directory
func (b *Broswer) DownloadDefaultDirectory() string {
	return b.cfg.DownloadDirectory
}
