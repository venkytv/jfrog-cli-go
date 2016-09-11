package commands

import (
	"github.com/jfrogdev/jfrog-cli-go/utils/ioutils"
	"encoding/json"
	"io/ioutil"
	"os"
	"archive/zip"
	"io"
	"errors"
	"path/filepath"
	"strings"
	"strconv"
	"github.com/jfrogdev/jfrog-cli-go/utils/cliutils"
	"github.com/jfrogdev/jfrog-cli-go/utils/cliutils/logger"
)

const VULNERABILITY = "__vuln"
const COMPONENT = "__comp"

func OfflineUpdate(flags *OfflineUpdatesFlags) error {
	vulnerabilities, components, err := getFilesList(flags)
	if err != nil {
		return err
	}
	xrayTempDir, err := getXrayTempDir()
	if err != nil {
		return err
	}
	if len(vulnerabilities) > 0 {
		logger.Logger.Info("Downloading vulnerabilities...")
		if err := saveData(xrayTempDir, "vuln", "", vulnerabilities); err != nil {
			return err
		}
	} else {
		logger.Logger.Info("There aren't new vulnerabilities.")
	}

	if len(components) > 0 {
		logger.Logger.Info("Downloading components...")
		if err := saveData(xrayTempDir, "comp", "", components); err != nil {
			return err
		}
	} else {
		logger.Logger.Info("There aren't new components.")
	}

	return nil
}

func getXrayTempDir() (string, error) {
	tempDir := os.TempDir()
	xrayDir := tempDir + "/jfrog/xray/"
	if err := os.MkdirAll(xrayDir, 0777); err != nil {
		cliutils.CheckError(err)
		return "", nil
	}
	return xrayDir, nil
}

func saveData(xrsyTmpdir, filesPrefix, logMsgPrefix string, urlsList []string) (err error) {
	dataDir, err := ioutil.TempDir(xrsyTmpdir, filesPrefix)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := os.RemoveAll(dataDir); cerr != nil && err == nil {
			err = cerr
		}
	}()
	for i, url := range urlsList {
		fileName := filesPrefix + strconv.Itoa(i) + ".json"
		logger.Logger.Info(logMsgPrefix + "Downloading " + url)
		ioutils.DownloadFile(url, dataDir, fileName, false, ioutils.HttpClientDetails{})
	}
	logger.Logger.Info("Zipping files.")
	err = zipFolderFiles(dataDir, filesPrefix + ".zip")
	if err != nil {
		return err
	}
	logger.Logger.Info("Done zipping files.")
	return nil
}

func getFilesList(flags *OfflineUpdatesFlags) ([]string, []string, error) {
	logger.Logger.Info("Getting updates...")
	headers := make(map[string]string)
	headers["X-Xray-License"] = flags.License
	httpClientDetails := ioutils.HttpClientDetails{
		Headers: headers,
	}
	resp, body, _, err := ioutils.SendGet(flags.Url, false, httpClientDetails)
	if resp.StatusCode != 200 {
		err := errors.New("xray response: " + resp.Status)
		cliutils.CheckError(err)
		return nil, nil, err
	}
	if err != nil {
		cliutils.CheckError(err)
		return nil, nil, err
	}
	var urls FilesList
	json.Unmarshal(body, &urls)
	var vulnerabilities, components []string
	for _, v := range urls.Urls {
		if strings.Contains(v, VULNERABILITY) {
			vulnerabilities = append(vulnerabilities, v)
		} else if strings.Contains(v, COMPONENT) {
			components = append(components, v)
		}
	}
	return vulnerabilities, components, nil
}

type OfflineUpdatesFlags struct {
	License string
	Url     string
}

type FilesList struct {
	Last_update int64
	Urls        []string
}

func zipFolderFiles(source, target string) (err error) {
	zipfile, err := os.Create(target)
	if err != nil {
		cliutils.CheckError(err)
		return
	}
	defer func() {
		if cerr := zipfile.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	archive := zip.NewWriter(zipfile)
	defer func() {
		if cerr := archive.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	filepath.Walk(source, func(path string, info os.FileInfo, err error) (currentErr error) {
		if info.IsDir() {
			return
		}

		if err != nil {
			currentErr = err
			return
		}

		header, currentErr := zip.FileInfoHeader(info)
		if currentErr != nil {
			cliutils.CheckError(currentErr)
			return
		}

		header.Method = zip.Deflate
		writer, currentErr := archive.CreateHeader(header)
		if currentErr != nil {
			cliutils.CheckError(currentErr)
			return
		}

		file, currentErr := os.Open(path)
		if currentErr != nil {
			cliutils.CheckError(currentErr)
			return
		}
		defer func() {
			if cerr := file.Close(); cerr != nil && currentErr == nil {
				currentErr = cerr
			}
		}()
		_, currentErr = io.Copy(writer, file)
		return
	})
	return
}