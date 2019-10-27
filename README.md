acpublisher
===========

acpublisher is a super simple command line tool for publishing apk files to
Microsoft AppCenter.

Currently the tool supports apk upload, release notes, publishing to groups and
ProGuard mapping files.
    
## Usage

    Usage: ./acpublisher <command> [<args>]
    Supported commands
        uploadApk    Upload APK to AppCenter

### uploadApk command help

    Usage: ./acpublisher uploadApk [<args>]
    Supported arguments
      -apk string
            Required. Path to apk file to upload
      -app string
            Required. Application name. This can be found from the web url: https://appcenter.ms/users/{owner}/apps/{app} or https://appcenter.ms/orgs/{owner}/apps/{app}
      -debug
            Optional. Enable debug logging
      -group value
            Optional. Id of the group where to distribute this release. Multiple groups can be set with multiple group arguments
      -mapping string
            Optional. Path to ProGuard mapping file to upload
      -owner string
            Required. Name of the application owner organization or user. This is can be found from the web url: https://appcenter.ms/users/{owner}/apps/{app} or https://appcenter.ms/orgs/{owner}/apps/{app}
      -releasenotes string
            Optional. Release notes (default "Uploaded with acpublisher")
      -releasenotesfile string
            Optional. Path to file containing release notes
      -token string
            Required. Api token for AppCenter
      -verbose
            Optional. Enable verbose logging

### Example call

    acpublisher uploadApk
                -token 1234567890123456789012345678901234567890 \
                -owner Example \
                -app ExampleApp \
                -apk /path/to/apk
