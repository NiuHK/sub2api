package service

import "testing"

func TestOwnedPrivateAccountUsesActualGroupMembership(t *testing.T) {
	const privateGroupID = int64(8)
	a := &Account{Platform: PlatformOpenAI, GroupIDs: []int64{privateGroupID}}
	if !ownedPrivateAccount(a, privateGroupID) {
		t.Fatal("owned account was not visible")
	}
	if ownedPrivateAccount(a, 9) {
		t.Fatal("different private group exposed account")
	}
	a.GroupIDs = nil
	a.AccountGroups = []AccountGroup{{GroupID: privateGroupID}}
	if !ownedPrivateAccount(a, privateGroupID) {
		t.Fatal("join association must grant access")
	}
	a.Platform = PlatformAnthropic
	if ownedPrivateAccount(a, privateGroupID) {
		t.Fatal("non-OpenAI account exposed")
	}
}
