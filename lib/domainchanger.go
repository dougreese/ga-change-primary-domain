package lib

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"google.golang.org/api/admin/directory/v1"
)

type DomainChanger struct {
	srv       *admin.Service
	cust      *admin.Customer
	oldDomain string
	newDomain string
}

func NewDomainChanger(client *http.Client, oldDomain string, newDomain string) (*DomainChanger, error) {
	srv, err := admin.New(client)
	if err != nil {
		return nil, err
	}

	dc := &DomainChanger{
		srv:       srv,
		oldDomain: oldDomain,
		newDomain: newDomain,
	}

	dc.GetCustomer()

	return dc, err
}

// checkNewDomain makes sure the new domain provided is not already the primary domain.
func (d *DomainChanger) checkNewDomain() bool {
	if d.cust.CustomerDomain == d.newDomain {
		return false
	}
	return true
}

// changeEmailDomain returns an email address using the provided domain.
func (d *DomainChanger) changeEmailDomain(email string, domain string) string {
	addrParts := strings.Split(email, "@")
	addrParts[1] = domain

	return strings.Join(addrParts, "@")
}

// emailNewDomain returns an email address using the new domain.
func (d *DomainChanger) emailNewDomain(email string) string {
	return d.changeEmailDomain(email, d.newDomain)
}

// emailOldDomain returns an email address using the old domain.
func (d *DomainChanger) emailOldDomain(email string) string {
	return d.changeEmailDomain(email, d.oldDomain)
}

// hasAlias checks to see if a given alias already exists in a given list of aliases.
func (d *DomainChanger) hasAlias(alias string, aliases []string) bool {
	for _, a := range aliases {
		if alias == a {
			return true
		}
	}
	return false
}

// GetCustomer retrieves customer data.
func (d *DomainChanger) GetCustomer() *admin.Customer {
	if nil == d.cust {
		cust, err := d.srv.Customers.Get("my_customer").Do()
		if err != nil {
			log.Fatalf("Unable to retrieve customer data.", err)
		}
		d.cust = cust
	}

	return d.cust
}

// ChangePrimaryDomain will change the primary domain for an account.
func (d *DomainChanger) ChangePrimaryDomain() {
	var code string

	// Make sure primary domain is not alrady newDomain
	if !d.checkNewDomain() {
		fmt.Printf("Primary domain for customer Id %s is already %s, continue checking users and groups? (y/n): ",
			d.cust.Id, d.newDomain)

		if _, err := fmt.Scan(&code); err != nil {
			log.Fatalf("Unable to read response %v", err)
		}
		if strings.ToLower(code) == "n" {
			log.Fatalln("Done.")
		}

		return
	}

	// Confirm changing primary domain
	fmt.Printf("About to update customer Id %s primary domain from %s to %s, continue? (y/n): ",
		d.cust.Id, d.cust.CustomerDomain, d.newDomain)
	if _, err := fmt.Scan(&code); err != nil {
		log.Fatalf("Unable to read response %v", err)
	}
	if strings.ToLower(code) != "y" {
		log.Fatalln("Abort!")
	}

	// Update customer domain
	fmt.Printf("\nUpdating customer Id %s primary domain from %s to %s ... ",
		d.cust.Id, d.cust.CustomerDomain, d.newDomain)

	custKey := d.cust.Id
	custUpdate := admin.Customer{
		CustomerDomain: d.newDomain,
	}
	_, err := d.srv.Customers.Update(custKey, &custUpdate).Do()
	if err != nil {
		log.Fatalf("Unable to update customer primary domain. %s", err)
	}
	fmt.Printf("Done.\n\n")

}

// UpdateUsers will rename all users in an account to the new domain.
func (d *DomainChanger) UpdateUsers() {
	ru, err := d.srv.Users.List().Customer("my_customer").MaxResults(50).
		OrderBy("email").Do()
	if err != nil {
		log.Fatalf("Unable to retrieve users in domain. %s", err)
	}

	if len(ru.Users) == 0 {
		fmt.Print("No users found.\n")
	} else {
		fmt.Print("\nUsers:\n")
		for _, u := range ru.Users {
			// fmt.Printf("%+v\n", u)

			emailUpdate := d.emailNewDomain(u.PrimaryEmail)
			if emailUpdate == u.PrimaryEmail {
				fmt.Printf("Email address for %s is already %s\n", u.Name.FullName, emailUpdate)
			} else {
				fmt.Printf("Changing primary domain for user: %s (%s) to %s ... ",
					u.PrimaryEmail, u.Name.FullName, emailUpdate)
				u.PrimaryEmail = emailUpdate

				u, err = d.srv.Users.Update(u.Id, u).Do()
				if err != nil {
					log.Fatalf("Unable to update user: %s - %s\n", u.Name.FullName, err)
				}
				fmt.Printf("Done.\n")
			}
			d.UpdateUserAliases(u)
		}
	}
}

// UpdateUserAliases will make sure an user email alias exists for the old primary domain.
func (d *DomainChanger) UpdateUserAliases(u *admin.User) {
	asrv := admin.NewUsersAliasesService(d.srv)
	oldAddress := d.emailOldDomain(u.PrimaryEmail)
	fmt.Printf(" Checking user %s email aliases on old domain %s ...\n", u.PrimaryEmail, d.oldDomain)

	for _, a := range u.Aliases {
		fmt.Printf(" - existing alias: %s\n", a)
		if a != oldAddress {
			// create new alias
			newA := d.emailNewDomain(a)
			if !d.hasAlias(newA, u.Aliases) {
				fmt.Printf(" - new alias: %s ... ", newA)
				aInsert := admin.Alias{
					Alias:        newA,
					PrimaryEmail: u.PrimaryEmail,
				}
				aConfirm, errA := asrv.Insert(u.Id, &aInsert).Do()
				if errA != nil {
					fmt.Printf(" Could not add new alias %s for user %s: %s\n", newA, u.PrimaryEmail, errA)
				} else {
					fmt.Printf(" New alias added for user %s: %s\n", aConfirm.PrimaryEmail, aConfirm.Alias)
				}
			}
		}
	}
	fmt.Printf(" Done checking user %s email aliases on %s.\n", u.PrimaryEmail, d.oldDomain)
}

// UpdateGroups will rename all groups in an account to the new domain.
func (d *DomainChanger) UpdateGroups() {
	// Check groups
	gu, err := d.srv.Groups.List().Customer(d.cust.Id).Do()
	if err != nil {
		log.Fatalf("Unable to retrieve groups in domain. %s", err)
	}

	if len(gu.Groups) == 0 {
		fmt.Print("No groups found.\n")
	} else {
		fmt.Print("\nGroups:\n")
		for _, g := range gu.Groups {
			// fmt.Printf("%+v\n", g)

			emailUpdate := d.emailNewDomain(g.Email)
			if emailUpdate == g.Email {
				fmt.Printf("Email address for %s is already %s\n", g.Name, emailUpdate)
			} else {
				fmt.Printf("Changing email address for group: %s (%s) to %s ... ",
					g.Email, g.Name, emailUpdate)
				g.Email = emailUpdate

				g, err := d.srv.Groups.Update(g.Id, g).Do()

				if err != nil {
					log.Fatalf("Unable to update group: %s - %s\n", g.Name, err)
				}
				fmt.Printf("Done.\n")
			}
			d.UpdateGroupAliases(g)
		}
	}
}

// UpdateGroupAliases will make sure an group email alias exists for the old primary domain.
func (d *DomainChanger) UpdateGroupAliases(g *admin.Group) {
	asrv := admin.NewGroupsAliasesService(d.srv)
	oldAddress := d.emailOldDomain(g.Email)
	fmt.Printf(" Checking group %s email aliases on old domain %s ...\n", g.Email, d.oldDomain)

	for _, a := range g.Aliases {
		fmt.Printf(" - existing alias: %s\n", a)
		if a != oldAddress {
			// create new alias
			newA := d.emailNewDomain(a)
			if !d.hasAlias(newA, g.Aliases) {
				fmt.Printf(" - new alias: %s ... ", newA)
				aInsert := admin.Alias{
					Alias:        newA,
					PrimaryEmail: g.Email,
				}
				aConfirm, errA := asrv.Insert(g.Id, &aInsert).Do()
				if errA != nil {
					fmt.Printf(" Could not add new alias %s for group %s: %s\n", newA, g.Email, errA)
				} else {
					fmt.Printf(" New alias added for group %s: %s\n", aConfirm.PrimaryEmail, aConfirm.Alias)
				}
			}
		}
	}
	fmt.Printf(" Done checking group %s email aliases on %s ...\n", g.Email, d.oldDomain)
}
