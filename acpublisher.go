package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/tkorri/acpublisher/command"
	"github.com/tkorri/acpublisher/logger"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
)

const versionString = "1.0.0"

const (
	baseURL    = "https://api.appcenter.ms"
	apiVersion = "v0.1"
)

const (
	uploadApkCmdString = "uploadApk"
)

const (
	Token            = "token"
	Owner            = "owner"
	App              = "app"
	Apk              = "apk"
	Mapping          = "mapping"
	ReleaseNotes     = "releasenotes"
	ReleaseNotesFile = "releasenotesfile"
	Group            = "group"
	Verbose          = "verbose"
	Debug            = "debug"
)

var log *logger.Logger

func main() {
	flag.Usage = showHelp
	flag.CommandLine.SetOutput(os.Stderr)

	// Upload apk
	uploadCommand := command.New(uploadApkCmdString)
	uploadCommand.AddString(Token, "", "Required. Api token for AppCenter")
	uploadCommand.AddString(Owner, "", "Required. Name of the application owner organization or user. This is can be found from the web url: https://appcenter.ms/users/{owner}/apps/{app} or https://appcenter.ms/orgs/{owner}/apps/{app}")
	uploadCommand.AddString(App, "", "Required. Application name. This can be found from the web url: https://appcenter.ms/users/{owner}/apps/{app} or https://appcenter.ms/orgs/{owner}/apps/{app}")
	uploadCommand.AddString(Apk, "", "Required. Path to apk file to upload")
	uploadCommand.AddString(Mapping, "", "Optional. Path to ProGuard mapping file to upload")
	uploadCommand.AddString(ReleaseNotes, "Uploaded with acpublisher", "Optional. Release notes")
	uploadCommand.AddString(ReleaseNotesFile, "", "Optional. Path to file containing release notes")
	uploadCommand.AddStringArray(Group, []string{}, "Optional. Id of the group where to distribute this release. Multiple groups can be set with multiple group arguments")
	uploadCommand.AddBool(Verbose, false, "Optional. Enable verbose logging")
	uploadCommand.AddBool(Debug, false, "Optional. Enable debug logging")

	if len(os.Args) < 2 {
		showHelp()
		os.Exit(1)
	}

	switch os.Args[1] {
	case uploadCommand.Name:
		handleUploadApkCommand(uploadCommand)
	default:
		showHelp()
		os.Exit(1)
	}
}

func showHelp() {
	logger.Errorln("acpublisher %s", versionString)
	logger.Errorln("Usage: %s <command> [<args>]", os.Args[0])
	logger.Errorln("Supported commands")
	logger.Errorln("    %s\tUpload Apk to AppCenter", uploadApkCmdString)
}

func showCommandHelp(command *command.Command) {
	logger.Errorln("acpublisher %s", versionString)
	logger.Errorln("Usage: %s %s [<args>]", os.Args[0], command.Name)
	logger.Errorln("Supported arguments")
	command.Command.PrintDefaults()
}

func jsonRequest(method string, url string, body interface{}, apiToken string, statusCode int, response interface{}) error {
	var bodyJson []byte
	var err error

	if body != nil {
		bodyJson, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}

	req, err := http.NewRequest(method, url, bytes.NewBuffer(bodyJson))
	if err != nil {
		return err
	}

	req.Header.Set("X-API-Token", apiToken)
	req.Header.Set("Content-Type", "application/json")

	log.V("--> %s %s %s", req.Method, req.URL.Path, req.Proto)
	printHeaders(req.Header)
	if bodyJson != nil {
		log.V("%s", string(bodyJson))
	}
	log.V("--> END %s", req.Method)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	log.V("<-- %s %s", resp.Proto, resp.Status)
	printHeaders(resp.Header)
	if responseBody != nil {
		log.V("%s", string(responseBody))
	}
	log.V("<-- END")

	if resp.StatusCode != statusCode {
		return errors.New(fmt.Sprintf("Unexpected response from server: %d", resp.StatusCode))
	}

	err = json.Unmarshal(responseBody, &response)
	if err != nil {
		return err
	}

	return nil
}

func printHeaders(header http.Header) {
	if len(header) > 0 {
		for name, values := range header {
			for _, value := range values {
				log.V("%s: %s", name, value)
			}
		}
	}
}

func handleVersionCommand() {
	logger.Println("acpublisher %s", versionString)
}

func handleUploadApkCommand(upload *command.Command) {
	err := upload.Command.Parse(os.Args[2:])
	if err != nil {
		logger.Errorln("Unrecognized parameters:\n%s", err)
		showCommandHelp(upload)
		os.Exit(1)
	}

	log = logger.New(upload.GetBool(Verbose), upload.GetBool(Verbose) || upload.GetBool(Debug))

	if len(os.Args[2:]) == 0 {
		showCommandHelp(upload)
		os.Exit(1)
	}

	if upload.GetString(Owner) == "" {
		log.E("Owner is required")
		os.Exit(1)
	}

	if upload.GetString(App) == "" {
		log.E("App is required")
		os.Exit(1)
	}

	appSlug := upload.GetString(Owner) + "/" + upload.GetString(App)

	// Check that apk file is available
	apkFile, err := os.Open(upload.GetString(Apk))
	if err != nil {
		log.E("Cannot open apk file:\n%s", err)
		os.Exit(1)
	}
	defer apkFile.Close()

	// Setup release notes
	var releaseNotes = upload.GetString(ReleaseNotes)
	if upload.GetString(ReleaseNotesFile) != "" {
		notes, err := ioutil.ReadFile(upload.GetString(ReleaseNotesFile))
		if err != nil {
			log.E("Cannot read release notes file contents:\n%s", err)
			os.Exit(1)
		}
		releaseNotes = string(notes)
	}

	// Check that mapping file is available if one is set
	var mappingFile *os.File = nil
	if upload.GetString(Mapping) != "" {
		mappingFile, err = os.Open(upload.GetString(Mapping))
		if err != nil {
			log.E("Cannot open mapping file:\n%s", err)
			os.Exit(1)
		}
		defer mappingFile.Close()
	}

	// Create release
	log.I("Creating new release...")
	begin, err := beginReleaseUpload(appSlug, upload.GetString(Token))
	if err != nil {
		log.E("Release FAILED\n%s", err)
		os.Exit(1)
	}
	err = uploadRelease(begin.UploadUrl, apkFile)
	if err != nil {
		log.E("Release FAILED\n%s", err)
		os.Exit(1)
	}
	response, err := commitRelease(appSlug, upload.GetString(Token), begin.UploadId)
	if err != nil {
		log.E("Release FAILED\n%s", err)
		os.Exit(1)
	}

	_, err = updateRelease(appSlug, upload.GetString(Token), response.ReleaseId, releaseNotes)
	if err != nil {
		log.E("Release FAILED")
		os.Exit(1)
	}
	log.I("Release %s OK", response.ReleaseId)

	// Publish release to groups
	if len(upload.GetStringArray(Group)) > 0 {
		log.I("Publishing release %s to group(s)...", response.ReleaseId)
		for _, group := range upload.GetStringArray(Group) {
			_, err = publishRelease(appSlug, upload.GetString(Token), response.ReleaseId, "groups", group)
			if err != nil {
				log.E("Publishing FAILED\n%s", err)
				os.Exit(1)
			}
		}
		log.I("Publish OK")
	} else {
		log.D("No groups defined, skipping publish")
	}

	// If mapping file is set and available then proceed with mapping upload
	if mappingFile != nil {
		log.I("Uploading mapping file...")

		release, err := getRelease(appSlug, upload.GetString(Token), response.ReleaseId)
		if err != nil {
			log.E("Uploading FAILED\n%s", err)
			os.Exit(1)
		}
		begin, err := beginSymbolUpload(appSlug, upload.GetString(Token), release.ShortVersion, release.Version, mappingFile)
		if err != nil {
			log.E("Uploading FAILED\n%s", err)
			os.Exit(1)
		}

		err = uploadSymbols(mappingFile, begin.UploadUrl)
		if err != nil {
			_, _ = commitSymbols(appSlug, upload.GetString(Token), begin.SymbolUploadId, ABORTED)
			log.E("Uploading FAILED\n%s", err)
			os.Exit(1)
		}

		_, err = commitSymbols(appSlug, upload.GetString(Token), begin.SymbolUploadId, COMMITTED)
		if err != nil {
			log.E("Uploading FAILED\n%s", err)
			os.Exit(1)
		}
		log.I("Mapping upload OK")
	} else {
		log.D("No mapping file defined, skipping mapping file upload")
	}
}

func beginReleaseUpload(appSlug string, apiToken string) (*ReleaseUploadBeginResponse, error) {
	uploadUrl := baseURL + "/" + apiVersion + "/apps/" + appSlug + "/release_uploads"
	log.D("Begin release upload")

	request := ReleaseUploadBeginRequest{}
	response := ReleaseUploadBeginResponse{}

	err := jsonRequest(http.MethodPost, uploadUrl, &request, apiToken, http.StatusCreated, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

func uploadRelease(uploadUrl string, file *os.File) error {
	log.D("Upload release %s", filepath.Base(file.Name()))

	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	fw, err := w.CreateFormFile("ipa", file.Name())
	if err != nil {
		return err
	}
	_, err = io.Copy(fw, file)
	if err != nil {
		return err
	}
	err = w.Close()
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, uploadUrl, &b)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", w.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return nil
}

func commitRelease(appSlug string, apiToken string, uploadId string) (*ReleaseUploadEndResponse, error) {
	commitUrl := baseURL + "/" + apiVersion + "/apps/" + appSlug + "/release_uploads/" + uploadId
	log.D("Commit release %s", uploadId)

	request := ReleaseUploadEndRequest{Status: COMMITTED}
	response := ReleaseUploadEndResponse{}

	err := jsonRequest(http.MethodPatch, commitUrl, &request, apiToken, http.StatusOK, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

func updateRelease(appSlug string, apiToken string, releaseId string, releaseNotes string) (*ReleaseUpdateResponse, error) {
	updateUrl := baseURL + "/" + apiVersion + "/apps/" + appSlug + "/releases/" + releaseId
	log.D("Update release %s", releaseId)

	request := ReleaseUpdateRequest{ReleaseNotes: releaseNotes}
	response := ReleaseUpdateResponse{}

	err := jsonRequest(http.MethodPut, updateUrl, &request, apiToken, http.StatusOK, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

func publishRelease(appSlug string, apiToken string, releaseId string, destinationType string, destinationId string) (*ReleaseDestinationResponse, error) {
	publishUrl := baseURL + "/" + apiVersion + "/apps/" + appSlug + "/releases/" + releaseId + "/" + destinationType
	log.D("Publishing to %s %s", destinationType, destinationId)

	request := ReleaseDestinationRequest{Id: destinationId}
	response := ReleaseDestinationResponse{}

	err := jsonRequest(http.MethodPost, publishUrl, &request, apiToken, http.StatusCreated, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil

}

func getRelease(appSlug string, apiToken string, releaseId string) (*ReleaseDetailsResponse, error) {
	releaseUrl := baseURL + "/" + apiVersion + "/apps/" + appSlug + "/releases/" + releaseId
	log.D("Get release %s", releaseId)

	response := ReleaseDetailsResponse{}

	err := jsonRequest(http.MethodGet, releaseUrl, nil, apiToken, http.StatusOK, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

func beginSymbolUpload(appSlug string, apiToken string, version string, build string, mappingFile *os.File) (*SymbolUploadBeginResponse, error) {
	uploadUrl := baseURL + "/" + apiVersion + "/apps/" + appSlug + "/symbol_uploads"
	log.D("Begin symbol upload")

	request := SymbolUploadBeginRequest{
		SymbolType: SymbolTypeAndroid,
		FileName:   filepath.Base(mappingFile.Name()),
		Version:    version,
		Build:      build,
	}
	response := SymbolUploadBeginResponse{}

	err := jsonRequest(http.MethodPost, uploadUrl, &request, apiToken, http.StatusOK, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

func uploadSymbols(mappingFile *os.File, uploadUrl string) error {
	log.D("Upload symbols")
	ctx := context.Background()

	parsedUrl, err := url.Parse(uploadUrl)
	if err != nil {
		return err
	}

	pipeline := azblob.NewPipeline(azblob.NewAnonymousCredential(), azblob.PipelineOptions{})

	blockBlobURL := azblob.NewBlockBlobURL(*parsedUrl, pipeline)

	_, err = azblob.UploadFileToBlockBlob(ctx, mappingFile, blockBlobURL, azblob.UploadToBlockBlobOptions{})
	if err != nil {
		return err
	}

	return nil
}

func commitSymbols(appSlug string, apiToken string, symbolUploadId string, status UploadStatus) (*SymbolUpload, error) {
	commitUrl := baseURL + "/" + apiVersion + "/apps/" + appSlug + "/symbol_uploads/" + symbolUploadId
	log.D("Commit symbols %s", symbolUploadId)

	request := SymbolUploadEndRequest{Status: status}
	response := SymbolUpload{}

	err := jsonRequest(http.MethodPatch, commitUrl, &request, apiToken, http.StatusOK, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

type ReleaseUploadBeginRequest struct {
	ReleaseId    int    `json:"release_id,omitempty"`
	BuildVersion string `json:"build_version,omitempty"`
	BuildNumber  string `json:"build_number,omitempty"`
}

type ReleaseUploadBeginResponse struct {
	UploadId    string `json:"upload_id"`
	UploadUrl   string `json:"upload_url"`
	AssetId     string `json:"asset_id,omitempty"`
	AssetDomain string `json:"asset_domain,omitempty"`
	AssetToken  string `json:"asset_token,omitempty"`
}

type UploadStatus string

const (
	COMMITTED UploadStatus = "committed"
	ABORTED   UploadStatus = "aborted"
)

type ReleaseUploadEndRequest struct {
	Status UploadStatus `json:"status"`
}

type ReleaseUploadEndResponse struct {
	ReleaseId  string `json:"release_id,omitempty"`
	ReleaseUrl string `json:"release_url,omitempty"`
}

type ReleaseUpdateRequest struct {
	ReleaseNotes    string `json:"release_notes,omitempty"`
	MandatoryUpdate bool   `json:"mandatory_update,omitempty"`
	Destinations    *[]struct {
		Id   string `json:"id,omitempty"`
		Name string `json:"name,omitempty"`
	} `json:"destinations,omitempty"`
	Build *struct {
		BranchName    string `json:"branch_name,omitempty"`
		CommitHash    string `json:"commit_hash,omitempty"`
		CommitMessage string `json:"commit_message,omitempty"`
	} `json:"build,omitempty"`
	NotifyTesters bool `json:"notify_testers,omitempty"`
	Metadata      *struct {
		DsaSignature string `json:"dsa_signature,omitempty"`
	} `json:"metadata,omitempty"`
}

type ReleaseUpdateResponse struct {
	Enabled               bool   `json:"enabled,omitempty"`
	MandatoryUpdate       bool   `json:"mandatory_update,omitempty"`
	ReleaseNotes          string `json:"release_notes,omitempty"`
	ProvisioningStatusUrl string `json:"provisioning_status_url,omitempty"`
	Destinations          *[]struct {
		Id   string `json:"id,omitempty"`
		Name string `json:"name,omitempty"`
	} `json:"destinations,omitempty"`
}

type ReleaseDestinationRequest struct {
	Id              string `json:"id"`
	MandatoryUpdate bool   `json:"mandatory_update,omitempty"`
	NotifyTesters   bool   `json:"notify_testers,omitempty"`
}

type ReleaseDestinationResponse struct {
	Id                    string `json:"id"`
	MandatoryUpdate       bool   `json:"mandatory_update"`
	ProvisioningStatusUrl string `json:"provisioning_status_url,omitempty"`
}

type SymbolType string

const (
	SymbolTypeApple      SymbolType = "Apple"
	SymbolTypeJavascript SymbolType = "JavaScript"
	SymbolTypeBreakpad   SymbolType = "Breakpad"
	SymbolTypeAndroid    SymbolType = "AndroidProguard"
	SymbolTypeUWP        SymbolType = "UWP"
)

type SymbolUploadBeginRequest struct {
	SymbolType     SymbolType `json:"symbol_type"`
	ClientCallback string     `json:"client_callback,omitempty"`
	FileName       string     `json:"file_name,omitempty"`
	Build          string     `json:"build,omitempty"`
	Version        string     `json:"version,omitempty"`
}

type SymbolUploadBeginResponse struct {
	SymbolUploadId string `json:"symbol_upload_id"`
	UploadUrl      string `json:"upload_url"`
	ExpirationDate string `json:"expiration_date"`
}

type SymbolUploadEndRequest struct {
	Status UploadStatus `json:"status"`
}

type SymbolUploadStatus string

const (
	SymbolUploadStatusCreated    SymbolUploadStatus = "created"
	SymbolUploadStatusCommitted  SymbolUploadStatus = "committed"
	SymbolUploadStatusAborted    SymbolUploadStatus = "aborted"
	SymbolUploadStatusProcessing SymbolUploadStatus = "processing"
	SymbolUploadStatusIndexed    SymbolUploadStatus = "indexed"
	SymbolUploadStatusFailed     SymbolUploadStatus = "failed"
)

type SymbolUpload struct {
	SymbolUploadId string `json:"symbol_upload_id"`
	AppId          string `json:"app_id"`
	User           *struct {
		Email       string `json:"email,omitempty"`
		DisplayName string `json:"display_name,omitempty"`
	} `json:"user,omitempty"`
	Status     SymbolUploadStatus `json:"status"`
	SymbolType SymbolType         `json:"symbol_type"`
}

type ReleaseDetailsResponse struct {
	Id             int    `json:"id"`
	AppName        string `json:"app_name"`
	AppDisplayName string `json:"app_display_name"`
	Version        string `json:"version"`
	ShortVersion   string `json:"short_version"`
	UploadedAt     string `json:"uploaded_at"`
	AppIconUrl     string `json:"app_icon_url"`
	Enabled        bool   `json:"enabled"`
}
