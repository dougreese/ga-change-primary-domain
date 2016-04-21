# Change Primary Domain in a Google Apps Account

This is a tool to change Google Apps primary domain and rename all users, using the [Google API Go client package](https://code.googlesource.com/google-api-go-client).

## Resources

* [Before you change your primary domain](https://support.google.com/a/answer/6301932/)
* Short [video](https://www.youtube.com/watch?v=G8GdNAZE98E) that uses the [APIs Explorer to change the primary domain](https://developers.google.com/apis-explorer/#p/admin/directory_v1/)
* [Google APIs Developer console](https://console.developers.google.com/project) - to set up a project and download your client_secret.json data
* [Google Directory API Go Quickstart](https://developers.google.com/admin-sdk/directory/v1/quickstart/go)
* [Documentation for API Go package](https://godoc.org/google.golang.org/api/admin/directory/v1)

## Usage

1. Set up your access to the Google API as described in the resources above
2. Download the client_secret.json file from the developer console, place in the same directory as this project
2. Build and run (or just run) the tool, passing the new and old domains as parameters
3. Go to the URL displayed, which will perform the proper auth, paste token into console when prompted
4. Confirm and off you go...

## Examples

```
go run changeprimarydomain.go -new-domain my-new-primary-domain.com -old-domain my-old-promary-domain.com
```

```
go build changeprimarydomain.go
./changeprimarydomain -new-domain my-new-primary-domain.com -old-domain my-old-promary-domain.com
```

## Notes

* Works on OS X and Linux, unknown if this will work properly on Windows
* Based on the [Google Directory API Go Quickstart](https://developers.google.com/admin-sdk/directory/v1/quickstart/go)
* To delete the cached auth credentials, remove the `~/.credentials/changeprimarydomain-<new domain>.json` file
  * Example: `rm ~/.credentials/changeprimarydomain-my-new-primary-domain.com.json`
* The 'Before you change your primary domain' document does not mention anything about renaming groups, but this console app does rename and alias all groups attached to the account
* This console app can be run multiple times for the same new and old domains with no negative ramifications
