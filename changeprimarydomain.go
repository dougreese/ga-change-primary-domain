package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path/filepath"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/admin/directory/v1"

	"./lib"
)

var newDomain string
var oldDomain string

func init() {
	flag.StringVar(&newDomain, "new-domain", "", "New primary domain")
	flag.StringVar(&oldDomain, "old-domain", "", "Old primary domain")
}

// getClient uses a Context and Config to retrieve a Token
// then generate a Client. It returns the generated Client.
func getClient(ctx context.Context, config *oauth2.Config) *http.Client {
	cacheFile, err := tokenCacheFile()
	if err != nil {
		log.Fatalf("Unable to get path to cached credential file. %v", err)
	}
	tok, err := tokenFromFile(cacheFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(cacheFile, tok)
	}
	return config.Client(ctx, tok)
}

// getTokenFromWeb uses Config to request a Token.
// It returns the retrieved Token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var code string
	if _, err := fmt.Scan(&code); err != nil {
		log.Fatalf("Unable to read authorization code %v", err)
	}

	tok, err := config.Exchange(oauth2.NoContext, code)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web %v", err)
	}
	return tok
}

// tokenCacheFile generates credential file path/filename.
// It returns the generated credential path/filename.
func tokenCacheFile() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	tokenCacheDir := filepath.Join(usr.HomeDir, ".credentials")
	os.MkdirAll(tokenCacheDir, 0700)
	return filepath.Join(tokenCacheDir,
		url.QueryEscape(fmt.Sprintf("changeprimarydomain-%s.json", newDomain))), err
}

// tokenFromFile retrieves a Token from a given file path.
// It returns the retrieved Token and any read error encountered.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	t := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(t)
	defer f.Close()
	return t, err
}

// saveToken uses a file path to create a file and store the
// token in it.
func saveToken(file string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", file)
	f, err := os.Create(file)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

// hasAlias checks to see if a user already has a given email alias.
func hasAlias(alias string, aliases []string) bool {
	for _, a := range aliases {
		if alias == a {
			return true
		}
	}
	return false
}

func main() {
	flag.Parse()

	if oldDomain == "" || newDomain == "" {
		flag.PrintDefaults()
		log.Fatal("Try again.")
	}
	log.Printf("Changing primary domain - old domain: %s, new domain: %s", oldDomain, newDomain)

	ctx := context.Background()

	b, err := ioutil.ReadFile("client_secret.json")
	if err != nil {
		log.Fatalf("Unable to read client_secret.json file, go here and download one: https://console.developers.google.com/project")
	}

	// If modifying these scopes, delete your previously saved credentials
	// at ~/.credentials/changeprimarydomain-<new domain>.json
	config, err := google.ConfigFromJSON(b,
		admin.AdminDirectoryUserScope,
		admin.AdminDirectoryCustomerScope,
		admin.AdminDirectoryGroupScope)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := getClient(ctx, config)

	dc, err := lib.NewDomainChanger(client, oldDomain, newDomain)
	if err != nil {
		log.Fatalf("Unable to initialize domain changer: %v", err)
	}

	dc.ChangePrimaryDomain()
	dc.UpdateUsers()
	dc.UpdateGroups()

	fmt.Printf("\nProcess complete.\n")
}
