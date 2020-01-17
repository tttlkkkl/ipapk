package ipapk // import "github.com/tttlkkkl/ipapk"

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/andrianbdn/iospng"
	"github.com/shogo82148/androidbinary"
	"github.com/shogo82148/androidbinary/apk"
	"howett.net/plist"
)

var (
	reInfoPlist = regexp.MustCompile(`Payload/[^/]+/Info\.plist`)
	// ErrNoIcon icon not found
	ErrNoIcon = errors.New("icon not found")
)

const (
	iosExt     = ".ipa"
	androidExt = ".apk"
)

// AppInfo 应用信息
type AppInfo struct {
	Name     string
	BundleID string
	Version  string
	Build    string
	Icon     image.Image
	Size     int64
}

type androidManifest struct {
	Package     string `xml:"package,attr"`
	VersionName string `xml:"versionName,attr"`
	VersionCode string `xml:"versionCode,attr"`
}

type iosPlist struct {
	CFBundleName         string `plist:"CFBundleName"`
	CFBundleDisplayName  string `plist:"CFBundleDisplayName"`
	CFBundleVersion      string `plist:"CFBundleVersion"`
	CFBundleShortVersion string `plist:"CFBundleShortVersionString"`
	CFBundleIDentifier   string `plist:"CFBundleIdentifier"`
}

// NewAppParser new
func NewAppParser(name string) (*AppInfo, error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}

	reader, err := zip.NewReader(file, stat.Size())
	if err != nil {
		return nil, err
	}

	var xmlFile, plistFile, iosIconFile, maxIosIconFile *zip.File
	var x, y int64
	for _, f := range reader.File {
		switch {
		case f.Name == "AndroidManifest.xml":
			xmlFile = f
		case reInfoPlist.MatchString(f.Name):
			plistFile = f
		case strings.Contains(f.Name, "AppIcon"):
			iosIconFile = f
			prexRegexp := regexp.MustCompile(`.+AppIcon-(\d{3}).+`)
			prexCom := prexRegexp.FindStringSubmatch(f.Name)

			if len(prexCom) == 2 {
				y, err = strconv.ParseInt(prexCom[1], 10, 64)
				if err != nil {
					x = 0
				}
				// 取最大的
				if y > x {
					x = y
					maxIosIconFile = f
				}
			}
		}
	}
	if maxIosIconFile != nil {
		iosIconFile = maxIosIconFile
	}
	ext := filepath.Ext(stat.Name())

	if ext == androidExt {
		info, err := parseApkFile(xmlFile)
		icon, label, err := parseApkIconAndLabel(name)
		info.Name = label
		info.Icon = icon
		info.Size = stat.Size()
		return info, err
	}

	if ext == iosExt {
		info, err := parseIpaFile(plistFile)
		icon, err := parseIpaIcon(iosIconFile)
		info.Icon = icon
		info.Size = stat.Size()
		return info, err
	}

	return nil, errors.New("unknown platform")
}

func parseAndroidManifest(xmlFile *zip.File) (*androidManifest, error) {
	rc, err := xmlFile.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	buf, err := ioutil.ReadAll(rc)
	if err != nil {
		return nil, err
	}

	xmlContent, err := androidbinary.NewXMLFile(bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}

	manifest := new(androidManifest)
	decoder := xml.NewDecoder(xmlContent.Reader())
	if err := decoder.Decode(manifest); err != nil {
		return nil, err
	}
	return manifest, nil
}

func parseApkFile(xmlFile *zip.File) (*AppInfo, error) {
	if xmlFile == nil {
		return nil, errors.New("AndroidManifest.xml not found")
	}

	manifest, err := parseAndroidManifest(xmlFile)
	if err != nil {
		return nil, err
	}

	info := new(AppInfo)
	info.BundleID = manifest.Package
	info.Version = manifest.VersionName
	info.Build = manifest.VersionCode

	return info, nil
}

func parseApkIconAndLabel(name string) (image.Image, string, error) {
	pkg, err := apk.OpenFile(name)
	if err != nil {
		return nil, "", err
	}
	defer pkg.Close()
	ar := &androidbinary.ResTableConfig{
		Density: 720,
	}
	icon, _ := pkg.Icon(ar)
	if icon == nil {
		return nil, "", ErrNoIcon
	}
	label, _ := pkg.Label(ar)
	return icon, label, nil
}

func parseIpaFile(plistFile *zip.File) (*AppInfo, error) {
	if plistFile == nil {
		return nil, errors.New("info.plist not found")
	}

	rc, err := plistFile.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	buf, err := ioutil.ReadAll(rc)
	if err != nil {
		return nil, err
	}
	p := new(iosPlist)
	decoder := plist.NewDecoder(bytes.NewReader(buf))
	if err := decoder.Decode(p); err != nil {
		return nil, err
	}

	info := new(AppInfo)
	if p.CFBundleDisplayName == "" {
		info.Name = p.CFBundleName
	} else {
		info.Name = p.CFBundleDisplayName
	}
	info.BundleID = p.CFBundleIDentifier
	info.Version = p.CFBundleShortVersion
	info.Build = p.CFBundleVersion

	return info, nil
}

func parseIpaIcon(iconFile *zip.File) (image.Image, error) {
	if iconFile == nil {
		return nil, ErrNoIcon
	}

	rc, err := iconFile.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	var w bytes.Buffer
	iospng.PngRevertOptimization(rc, &w)

	return png.Decode(bytes.NewReader(w.Bytes()))
}

// Lookup https://itunes.apple.com/lookup?bundleId=BundleID
type Lookup struct {
	RsultCount int64          `json:"resultCount"`
	Results    []LookupResult `json:"results"`
}

// LookupResult 结果
type LookupResult struct {
	ScreenshotUrls        []string `json:"screenshotUrls"`
	IpadScreenshotUrls    []string `json:"ipadScreenshotUrls"`
	AppletvScreenshotUrls []string `json:"appletvScreenshotUrls"`
	ArtworkURL60          string   `json:"artworkUrl60"`
	ArtworkURL512         string   `json:"artworkUrl512"`
	ArtworkURL100         string   `json:"artworkUrl100"`
	ArtistViewURL         string   `json:"artistViewUrl"`
	SupportedDevices      []string `json:"supportedDevices"`
	LanguageCodesISO2A    []string `json:"languageCodesISO2A"`
	TrackViewURL          string   `json:"trackViewUrl"`
}

// StoreURL app store 地址
type StoreURL string

func (s *StoreURL) String() string {
	ss := *s
	return string(ss)
}

// Cn 中国区地址
func (s *StoreURL) Cn() string {
	u, err := url.Parse(s.String())
	if err != nil {
		return s.String()
	}
	p := strings.Split(strings.TrimSpace(u.Path), "/")
	if len(p) > 2 {
		p = p[2:]
		p = append([]string{"cn"}, p...)
	}
	u.Path = path.Join(p...)
	return u.String()
}

// GetIosAppStoreAddress 获取app store地址
func GetIosAppStoreAddress(bundleID string) StoreURL {
	lk, err := GetLookup(bundleID)
	if err != nil {
		return StoreURL("")
	}
	if lk.RsultCount > 0 {
		return StoreURL(lk.Results[0].TrackViewURL)
	}
	return StoreURL("")
}

// GetLookup 获取应用商店信息
func GetLookup(bundleID string) (*Lookup, error) {
	var lk Lookup
	resp, err := http.DefaultClient.Get(fmt.Sprintf("https://itunes.apple.com/lookup?bundleId=%s", bundleID))
	if err != nil {
		return &lk, err
	}
	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return &lk, err
	}
	if err = json.Unmarshal(bytes, &lk); err != nil {
		return &lk, err
	}
	return &lk, nil
}
