package auth_test

import (
	"errors"
	"testing"

	"github.com/influxdata/kapacitor/auth"
)

func Test_Privilege_String(t *testing.T) {
	testCases := []struct {
		p auth.Privilege
		s string
	}{
		{
			p: auth.NoPrivileges,
			s: "NO_PRIVILEGES",
		},
		{
			p: auth.GETPrivilege,
			s: "GET",
		},
		{
			p: auth.POSTPrivilege,
			s: "POST",
		},
		{
			p: auth.PATCHPrivilege,
			s: "PATCH",
		},
		{
			p: auth.DELETEPrivilege,
			s: "DELETE",
		},
		{
			p: auth.AllPrivileges,
			s: "ALL_PRIVILEGES",
		},
		{
			p: auth.AllPrivileges + 1,
			s: "UNKNOWN_PRIVILEGE",
		},
	}

	for _, tc := range testCases {
		if exp, got := tc.s, tc.p.String(); exp != got {
			t.Errorf("unexpected string value: got %s exp %s", got, exp)
		}
	}
}

func Test_Action_RequiredPrilege(t *testing.T) {
	testCases := []struct {
		a   auth.Action
		rp  auth.Privilege
		err error
	}{
		{
			a: auth.Action{
				Resource: "/",
				Method:   "GET",
			},
			rp:  auth.GETPrivilege,
			err: nil,
		},
		{
			a: auth.Action{
				Resource: "/",
				Method:   "get",
			},
			rp:  auth.GETPrivilege,
			err: nil,
		},
		{
			a: auth.Action{
				Resource: "/",
				Method:   "HEAD",
			},
			rp:  auth.GETPrivilege,
			err: nil,
		},
		{
			a: auth.Action{
				Resource: "/",
				Method:   "head",
			},
			rp:  auth.GETPrivilege,
			err: nil,
		},
		{
			a: auth.Action{
				Resource: "/",
				Method:   "OPTIONS",
			},
			rp:  auth.GETPrivilege,
			err: nil,
		},
		{
			a: auth.Action{
				Resource: "/",
				Method:   "options",
			},
			rp:  auth.GETPrivilege,
			err: nil,
		},
		{
			a: auth.Action{
				Resource: "/",
				Method:   "POST",
			},
			rp:  auth.POSTPrivilege,
			err: nil,
		},
		{
			a: auth.Action{
				Resource: "/",
				Method:   "post",
			},
			rp:  auth.POSTPrivilege,
			err: nil,
		},
		{
			a: auth.Action{
				Resource: "/",
				Method:   "PATCH",
			},
			rp:  auth.PATCHPrivilege,
			err: nil,
		},
		{
			a: auth.Action{
				Resource: "/",
				Method:   "patch",
			},
			rp:  auth.PATCHPrivilege,
			err: nil,
		},
		{
			a: auth.Action{
				Resource: "/",
				Method:   "DELETE",
			},
			rp:  auth.DELETEPrivilege,
			err: nil,
		},
		{
			a: auth.Action{
				Resource: "/",
				Method:   "delete",
			},
			rp:  auth.DELETEPrivilege,
			err: nil,
		},
		{
			a: auth.Action{
				Resource: "/",
				Method:   "PUT",
			},
			err: errors.New(`unknown method "PUT"`),
		},
	}

	for _, tc := range testCases {
		got, err := tc.a.RequiredPrivilege()
		if err != nil {
			if tc.err == nil {
				t.Errorf("unexpected error: got %v", err)
			} else if tc.err.Error() != err.Error() {
				t.Errorf("unexpected error message: got %q exp %q", err.Error(), tc.err.Error())
			}
		} else {
			if tc.err != nil {
				t.Errorf("expected error: %q got nil", tc.err.Error())
				continue
			}
			if got != tc.rp {
				t.Errorf("unexpected required privilege: got %v exp %v", got, tc.rp)
			}
		}
	}
}
func Test_User_Name(t *testing.T) {
	u := auth.NewUser("username", nil, nil)
	if got := u.Name(); got != "username" {
		t.Errorf("unexpected username: got %s exp username", got)
	}
}

func Test_User_AuthorizeAction(t *testing.T) {
	testCases := []struct {
		actionPrivileges map[string]auth.Privilege
		action           auth.Action
		authorized       bool
		err              error
	}{
		{
			actionPrivileges: map[string]auth.Privilege{
				"/a/b/c": auth.POSTPrivilege,
			},
			action: auth.Action{
				Resource: "/a/b/c",
				Method:   "POST",
			},
			authorized: true,
			err:        nil,
		},
		{
			actionPrivileges: map[string]auth.Privilege{
				"/a/b/": auth.POSTPrivilege,
			},
			action: auth.Action{
				Resource: "/a/b/c",
				Method:   "POST",
			},
			authorized: true,
			err:        nil,
		},
		{
			actionPrivileges: map[string]auth.Privilege{
				"/a/": auth.POSTPrivilege,
			},
			action: auth.Action{
				Resource: "/a/b/c",
				Method:   "POST",
			},
			authorized: true,
			err:        nil,
		},
		{
			actionPrivileges: map[string]auth.Privilege{
				"/": auth.POSTPrivilege,
			},
			action: auth.Action{
				Resource: "/a/b/c",
				Method:   "POST",
			},
			authorized: true,
			err:        nil,
		},
		{
			actionPrivileges: map[string]auth.Privilege{
				"/c/": auth.POSTPrivilege,
			},
			action: auth.Action{
				Resource: "/a/b/c",
				Method:   "POST",
			},
			authorized: false,
			err:        errors.New(`user bob does not have POST privilege for resource "/a/b/c"`),
		},
		{
			actionPrivileges: map[string]auth.Privilege{
				"/a/b/c/": auth.POSTPrivilege,
			},
			action: auth.Action{
				Resource: "/a/b/c",
				Method:   "POST",
			},
			authorized: false,
			err:        errors.New(`user bob does not have POST privilege for resource "/a/b/c"`),
		},
	}
	for _, tc := range testCases {
		u := auth.NewUser("bob", tc.actionPrivileges, nil)
		authorized, err := u.AuthorizeAction(tc.action)
		if err != nil {
			if tc.err == nil {
				t.Errorf("unexpected error authorizing action: got %q", err.Error())
			} else if err.Error() != tc.err.Error() {
				t.Errorf("unexpected error message: got %q exp %q", err.Error(), tc.err.Error())
			}
		} else {
			if tc.err != nil {
				t.Errorf("expected error authorizing action: %q", tc.err.Error())
			}
			if authorized != tc.authorized {
				t.Errorf("AUTH BREACH: got %t exp %t", authorized, tc.authorized)
			}
		}
	}
}
