package auth

import (
	"fmt"
	"log"
	"path"
	"strings"

	"github.com/influxdata/influxdb/influxql"
	"github.com/pkg/errors"
)

// Interface for authenticating and retrieving users.
type Interface interface {
	Authenticate(username, password string) (User, error)
	User(username string) (User, error)
}

type Privilege int

const (
	NoPrivileges Privilege = 1 << iota

	GETPrivilege
	POSTPrivilege
	PATCHPrivilege
	DELETEPrivilege

	AllPrivileges
)

func (p Privilege) String() string {
	switch p {
	case NoPrivileges:
		return "NO_PRIVILEGES"
	case GETPrivilege:
		return "GET"
	case POSTPrivilege:
		return "POST"
	case PATCHPrivilege:
		return "PATCH"
	case DELETEPrivilege:
		return "DELETE"
	case AllPrivileges:
		return "ALL_PRIVILEGES"
	default:
		return "UNKNOWN_PRIVILEGE"
	}
}

type Action struct {
	Resource string
	Method   string
}

func (a Action) RequiredPrivilege() (Privilege, error) {
	switch m := strings.ToUpper(a.Method); m {
	case "HEAD", "OPTIONS", "GET":
		return GETPrivilege, nil
	case "POST":
		return POSTPrivilege, nil
	case "PATCH":
		return PATCHPrivilege, nil
	case "DELETE":
		return DELETEPrivilege, nil
	default:
		return NoPrivileges, fmt.Errorf("unknown method %q", m)
	}
}

type User struct {
	name  string
	admin bool
	// Map of resource -> Bitmask of Privileges
	actionPrivileges map[string]Privilege
	// Map of databas -> influxql.Privilege
	dbPrivileges map[string]influxql.Privilege
}

// Create a user with the given privileges.
func NewUser(name string, actionPrivileges map[string]Privilege, dbPrivileges map[string]influxql.Privilege) User {
	return User{
		name:             name,
		actionPrivileges: actionPrivileges,
		dbPrivileges:     dbPrivileges,
	}
}

// Create a super user.
func NewAdminUser(name string) User {
	return User{
		name:  name,
		admin: true,
	}
}

// This user has all privileges for all resources.
var AdminUser = NewAdminUser("ADMIN_USER")

// Determine wether the user is authorized to take the action.
func (u User) Name() string {
	return u.name
}

// Determine wether the user is authorized to take the action.
func (u User) AuthorizeAction(action Action) (bool, error) {
	rp, err := action.RequiredPrivilege()
	if err != nil {
		return false, errors.Wrap(err, "cannot authorize invalid action")
	}
	if rp == NoPrivileges {
		return false, errors.New("cannot authorize invalid action")
	}
	if u.admin {
		return true, nil
	}
	// Find a matching resource of the form /path/to/resource
	// where if the resource is /a/b/c and the user has permision to /a/b
	// then it is considered valid.
	if !path.IsAbs(action.Resource) {
		return false, fmt.Errorf("invalid action resource: %q, must be an absolute path", action.Resource)
	}
	if len(u.actionPrivileges) > 0 {
		resource := action.Resource
		for resource != "" {
			log.Println("resource", resource)
			if p, ok := u.actionPrivileges[resource]; ok {
				// Found matching resource
				authorized := p&rp != 0 || p == AllPrivileges
				if authorized {
					return true, nil
				} else {
					break
				}
			}
			// Pop off the last piece of the resource and try again
			resource, _ = path.Split(strings.TrimSuffix(resource, "/"))
		}
	}
	return false, fmt.Errorf("user %s does not have %v privilege for resource %q", u.name, rp, action.Resource)
}

// Authorize returns true if the user is authorized and false if not.
func (u User) AuthorizeDB(privilege influxql.Privilege, database string) bool {
	if u.admin {
		return true
	}
	if len(u.dbPrivileges) == 0 {
		return false
	}
	p, ok := u.dbPrivileges[database]
	return ok && (p == privilege || p == influxql.AllPrivileges)
}
