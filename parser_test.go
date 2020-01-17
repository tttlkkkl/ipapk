package ipapk

import (
	"archive/zip"
	"bytes"
	"image/png"
	"os"
	"strings"
	"testing"
)

func getAppZipReader(filename string) (*zip.Reader, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}

	reader, err := zip.NewReader(file, stat.Size())
	if err != nil {
		return nil, err
	}
	return reader, nil
}

func getAndroidManifest() (*zip.File, error) {
	reader, err := getAppZipReader("testdata/helloworld.apk")
	if err != nil {
		return nil, err
	}
	var xmlFile *zip.File
	for _, f := range reader.File {
		if f.Name == "AndroidManifest.xml" {
			xmlFile = f
			break
		}
	}
	return xmlFile, nil
}

func TestParseAndroidManifest(t *testing.T) {
	xmlFile, err := getAndroidManifest()
	if err != nil {
		t.Errorf("got %v want no error", err)
	}
	manifest, err := parseAndroidManifest(xmlFile)
	if err != nil {
		t.Errorf("got %v want no error", err)
	}
	if manifest.Package != "com.example.helloworld" {
		t.Errorf("got %v want %v", manifest.Package, "com.example.helloworld")
	}
	if manifest.VersionName != "1.0" {
		t.Errorf("got %v want %v", manifest.VersionName, "1.0")
	}
	if manifest.VersionCode != "1" {
		t.Errorf("got %v want %v", manifest.VersionCode, "1")
	}
}

func TestParseApkFile(t *testing.T) {
	xmlFile, err := getAndroidManifest()
	if err != nil {
		t.Errorf("got %v want no error", err)
	}
	apk, err := parseApkFile(xmlFile)
	if err != nil {
		t.Errorf("got %v want no error", err)
	}
	if apk.BundleID != "com.example.helloworld" {
		t.Errorf("got %v want %v", apk.BundleID, "com.example.helloworld")
	}
	if apk.Version != "1.0" {
		t.Errorf("got %v want %v", apk.Version, "1.0")
	}
	if apk.Build != "1" {
		t.Errorf("got %v want %v", apk.Build, "1")
	}
}

func TestParseApkIconAndLabel(t *testing.T) {
	icon, label, err := parseApkIconAndLabel("testdata/helloworld.apk")
	if err != nil {
		t.Errorf("got %v want no error", err)
	}
	buf := new(bytes.Buffer)
	if err := png.Encode(buf, icon); err != nil {
		t.Errorf("got %v want no error", err)
	}
	if len(buf.Bytes()) != 10223 {
		t.Errorf("got %v want %v", len(buf.Bytes()), 10223)
	}
	if label != "HelloWorld" {
		t.Errorf("got %v want %v", label, "HelloWorld")
	}
}

func getIosPlist() (*zip.File, error) {
	reader, err := getAppZipReader("testdata/helloworld.ipa")
	if err != nil {
		return nil, err
	}
	var plistFile *zip.File
	for _, f := range reader.File {
		if reInfoPlist.MatchString(f.Name) {
			plistFile = f
			break
		}
	}
	return plistFile, nil
}

func TestParseIpaFile(t *testing.T) {
	plistFile, err := getIosPlist()
	if err != nil {
		t.Errorf("got %v want no error", err)
	}
	ipa, err := parseIpaFile(plistFile)
	if err != nil {
		t.Errorf("got %v want no error", err)
	}
	if ipa.BundleID != "com.kthcorp.helloworld" {
		t.Errorf("got %v want %v", ipa.BundleID, "com.kthcorp.helloworld")
	}
	if ipa.Version != "1.0" {
		t.Errorf("got %v want %v", ipa.Version, "1.0")
	}
	if ipa.Build != "1.0" {
		t.Errorf("got %v want %v", ipa.Build, "1.0")
	}
}

func TestParseIpaIcon(t *testing.T) {
	reader, err := getAppZipReader("testdata/helloworld.ipa")
	if err != nil {
		t.Errorf("got %v want no error", err)
	}
	var iconFile *zip.File
	for _, f := range reader.File {
		if strings.Contains(f.Name, "AppIcon60x60") {
			iconFile = f
			break
		}
	}
	if _, err := parseIpaIcon(iconFile); err != ErrNoIcon {
		t.Errorf("got %v want %v", err, ErrNoIcon)
	}
}

func TestStoreURL_Cn(t *testing.T) {
	tests := []struct {
		name string
		s    StoreURL
		want string
	}{
		{
			name: "test url 1",
			s:    StoreURL("https://apps.apple.com/us/app/teamkit/id1421743514?uo=4"),
			want: "https://apps.apple.com/cn/app/teamkit/id1421743514?uo=4",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.s.Cn(); got != tt.want {
				t.Errorf("StoreURL.Cn() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetIosAppStoreAddress(t *testing.T) {
	type args struct {
		bundleID string
	}
	tests := []struct {
		name string
		args args
		want StoreURL
	}{
		{
			name: "test 1",
			args: args{"com.ysdn.EPTool"},
			want: StoreURL("https://apps.apple.com/us/app/teamkit/id1421743514?uo=4"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetIosAppStoreAddress(tt.args.bundleID); got != tt.want {
				t.Errorf("GetIosAppStoreAddress() = %v, want %v", got, tt.want)
			}
		})
	}
}
