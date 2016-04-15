package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/admin/directory/v1"
)

var newDomain string

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

func main() {
	if len(os.Args) < 2 {
		log.Fatalln("Pass new domain name as the only parameter.")
	}
	newDomain = os.Args[1]

	ctx := context.Background()

	b, err := ioutil.ReadFile("client_secret.json")
	if err != nil {
		log.Fatalf("Unable to read client_secret.json file, go here and download one: https://console.developers.google.com/project")
	}

	// If modifying these scopes, delete your previously saved credentials
	// at ~/.credentials/changeprimarydomain-<new domain>.json
	config, err := google.ConfigFromJSON(b,
		admin.AdminDirectoryUserScope,
		admin.AdminDirectoryCustomerScope)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := getClient(ctx, config)

	srv, err := admin.New(client)
	if err != nil {
		log.Fatalf("Unable to retrieve directory Client %v", err)
	}

	// Retrieve customer/domain data
	cust, err := srv.Customers.Get("my_customer").Do()
	if err != nil {
		log.Fatalf("Unable to retrieve customer data.", err)
	}

	// Confirm changing primary domain
	fmt.Printf("About to update customer Id %s primary domain from %s to %s, continue? (y/n): ",
		cust.Id, cust.CustomerDomain, newDomain)
	var code string
	if _, err := fmt.Scan(&code); err != nil {
		log.Fatalf("Unable to read authorization code %v", err)
	}
	if strings.ToLower(code) != "y" {
		log.Fatalln("Abort!")
	}

	// Update customer domain
	fmt.Printf("\nUpdating customer Id %s primary domain from %s to %s ... ", cust.Id, cust.CustomerDomain, newDomain)

	custKey := cust.Id
	custUpdate := admin.Customer{
		CustomerDomain: newDomain,
	}
	cust, err = srv.Customers.Update(custKey, &custUpdate).Do()
	if err != nil {
		log.Fatalf("Unable to update customer primary domain. %s", err)
	}
	fmt.Printf("Done.\n\n")

	// Update all users
	ru, err := srv.Users.List().Customer("my_customer").MaxResults(50).
		OrderBy("email").Do()
	if err != nil {
		log.Fatalf("Unable to retrieve users in domain. %s", err)
	}

	if len(ru.Users) == 0 {
		fmt.Print("No users found.\n")
	} else {
		fmt.Print("Users:\n")
		for _, u := range ru.Users {
			// fmt.Printf("%+v\n", u)

			addrParts := strings.Split(u.PrimaryEmail, "@")
			addrParts[1] = newDomain
			emailUpdate := strings.Join(addrParts, "@")
			fmt.Printf("Changing primary domain for: %s (%s) to %s ... ", u.PrimaryEmail, u.Name.FullName, emailUpdate)
			u.PrimaryEmail = emailUpdate

			u2, err2 := srv.Users.Update(u.Id, u).Do()
			if err2 != nil {
				log.Fatalf("Unable to update user: %s - %s\n", u2.Name.FullName, err2)
			}
			fmt.Printf("Done.\n")
		}
	}

	fmt.Printf("\nProcess complete.\n")
}
