package vault

import (
	"reflect"
	"sort"
	"testing"

	"github.com/hashicorp/vault/helper/identity"
	"github.com/hashicorp/vault/logical"
)

func TestIdentityStore_MemDBGroupIndexes(t *testing.T) {
	var err error
	i, _, _ := testIdentityStoreWithGithubAuth(t)

	// Create a dummy group
	group := &identity.Group{
		ID:   "testgroupid",
		Name: "testgroupname",
		Metadata: map[string]string{
			"testmetadatakey1": "testmetadatavalue1",
			"testmetadatakey2": "testmetadatavalue2",
		},
		ParentGroupIDs:  []string{"testparentgroupid1", "testparentgroupid2"},
		MemberEntityIDs: []string{"testentityid1", "testentityid2"},
		Policies:        []string{"testpolicy1", "testpolicy2"},
		BucketKeyHash:   i.groupPacker.BucketKeyHashByItemID("testgroupid"),
	}

	// Insert it into memdb
	err = i.memDBUpsertGroup(group)
	if err != nil {
		t.Fatal(err)
	}

	// Insert another dummy group
	group = &identity.Group{
		ID:   "testgroupid2",
		Name: "testgroupname2",
		Metadata: map[string]string{
			"testmetadatakey2": "testmetadatavalue2",
			"testmetadatakey3": "testmetadatavalue3",
		},
		ParentGroupIDs:  []string{"testparentgroupid2", "testparentgroupid3"},
		MemberEntityIDs: []string{"testentityid2", "testentityid3"},
		Policies:        []string{"testpolicy2", "testpolicy3"},
		BucketKeyHash:   i.groupPacker.BucketKeyHashByItemID("testgroupid2"),
	}

	// Insert it into memdb
	err = i.memDBUpsertGroup(group)
	if err != nil {
		t.Fatal(err)
	}

	var fetchedGroup *identity.Group

	// Fetch group given the name
	fetchedGroup, err = i.memDBGroupByName("testgroupname", false)
	if err != nil {
		t.Fatal(err)
	}
	if fetchedGroup == nil || fetchedGroup.Name != "testgroupname" {
		t.Fatalf("failed to fetch an indexed group")
	}

	// Fetch group given the ID
	fetchedGroup, err = i.memDBGroupByID("testgroupid", false)
	if err != nil {
		t.Fatal(err)
	}
	if fetchedGroup == nil || fetchedGroup.Name != "testgroupname" {
		t.Fatalf("failed to fetch an indexed group")
	}

	var fetchedGroups []*identity.Group
	// Fetch the subgroups of a given group ID
	fetchedGroups, err = i.memDBGroupsByParentGroupID("testparentgroupid1", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(fetchedGroups) != 1 || fetchedGroups[0].Name != "testgroupname" {
		t.Fatalf("failed to fetch an indexed group")
	}

	fetchedGroups, err = i.memDBGroupsByParentGroupID("testparentgroupid2", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(fetchedGroups) != 2 {
		t.Fatalf("failed to fetch a indexed groups")
	}

	// Fetch groups based on policy name
	fetchedGroups, err = i.memDBGroupsByPolicy("testpolicy1", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(fetchedGroups) != 1 || fetchedGroups[0].Name != "testgroupname" {
		t.Fatalf("failed to fetch an indexed group")
	}

	fetchedGroups, err = i.memDBGroupsByPolicy("testpolicy2", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(fetchedGroups) != 2 {
		t.Fatalf("failed to fetch indexed groups")
	}

	// Fetch groups based on member entity ID
	fetchedGroups, err = i.memDBGroupsByMemberEntityID("testentityid1", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(fetchedGroups) != 1 || fetchedGroups[0].Name != "testgroupname" {
		t.Fatalf("failed to fetch an indexed group")
	}

	fetchedGroups, err = i.memDBGroupsByMemberEntityID("testentityid2", false)
	if err != nil {
		t.Fatal(err)
	}

	if len(fetchedGroups) != 2 {
		t.Fatalf("failed to fetch groups by entity ID")
	}
}

func TestIdentityStore_GroupsCreateUpdate(t *testing.T) {
	var resp *logical.Response
	var err error
	is, _, _ := testIdentityStoreWithGithubAuth(t)

	// Create an entity and get its ID
	entityRegisterReq := &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "entity",
	}
	resp, err = is.HandleRequest(entityRegisterReq)
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("bad: resp: %#v, err: %v", resp, err)
	}
	entityID1 := resp.Data["id"].(string)

	// Create another entity and get its ID
	resp, err = is.HandleRequest(entityRegisterReq)
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("bad: resp: %#v, err: %v", resp, err)
	}
	entityID2 := resp.Data["id"].(string)

	// Create a group with the above created 2 entities as its members
	groupData := map[string]interface{}{
		"policies":          "testpolicy1,testpolicy2",
		"metadata":          []string{"testkey1=testvalue1", "testkey2=testvalue2"},
		"member_entity_ids": []string{entityID1, entityID2},
	}

	// Create a group and get its ID
	groupReq := &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "group",
		Data:      groupData,
	}
	resp, err = is.HandleRequest(groupReq)
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("bad: resp: %#v, err: %v", resp, err)
	}
	memberGroupID1 := resp.Data["id"].(string)

	// Create another group and get its ID
	resp, err = is.HandleRequest(groupReq)
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("bad: resp: %#v, err: %v", resp, err)
	}
	memberGroupID2 := resp.Data["id"].(string)

	// Create a group with the above 2 groups as its members
	groupData["member_group_ids"] = []string{memberGroupID1, memberGroupID2}
	resp, err = is.HandleRequest(groupReq)
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("bad: resp: %#v, err: %v", resp, err)
	}
	groupID := resp.Data["id"].(string)

	// Read the group using its iD and check if all the fields are properly
	// set
	groupReq = &logical.Request{
		Operation: logical.ReadOperation,
		Path:      "group/id/" + groupID,
	}
	resp, err = is.HandleRequest(groupReq)
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("bad: resp: %#v, err: %v", resp, err)
	}

	expectedData := map[string]interface{}{
		"policies": []string{"testpolicy1", "testpolicy2"},
		"metadata": map[string]string{
			"testkey1": "testvalue1",
			"testkey2": "testvalue2",
		},
	}
	expectedData["id"] = resp.Data["id"]
	expectedData["name"] = resp.Data["name"]
	expectedData["member_group_ids"] = resp.Data["member_group_ids"]
	expectedData["member_entity_ids"] = resp.Data["member_entity_ids"]
	expectedData["creation_time"] = resp.Data["creation_time"]
	expectedData["last_update_time"] = resp.Data["last_update_time"]
	expectedData["modify_index"] = resp.Data["modify_index"]

	if !reflect.DeepEqual(expectedData, resp.Data) {
		t.Fatalf("bad: group data;\nexpected: %#v\n actual: %#v\n", expectedData, resp.Data)
	}

	// Update the policies and metadata in the group
	groupReq.Operation = logical.UpdateOperation
	groupReq.Data = groupData

	// Update by setting ID in the param
	groupData["id"] = groupID
	groupData["policies"] = "updatedpolicy1,updatedpolicy2"
	groupData["metadata"] = []string{"updatedkey=updatedvalue"}
	resp, err = is.HandleRequest(groupReq)
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("bad: resp: %#v, err: %v", resp, err)
	}

	// Check if updates are reflected
	groupReq.Operation = logical.ReadOperation
	resp, err = is.HandleRequest(groupReq)
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("bad: resp: %#v, err: %v", resp, err)
	}

	expectedData["policies"] = []string{"updatedpolicy1", "updatedpolicy2"}
	expectedData["metadata"] = map[string]string{
		"updatedkey": "updatedvalue",
	}
	expectedData["last_update_time"] = resp.Data["last_update_time"]
	expectedData["modify_index"] = resp.Data["modify_index"]
	if !reflect.DeepEqual(expectedData, resp.Data) {
		t.Fatalf("bad: group data; expected: %#v\n actual: %#v\n", expectedData, resp.Data)
	}
}

func TestIdentityStore_GroupsCRUD_ByID(t *testing.T) {
	var resp *logical.Response
	var err error
	is, _, _ := testIdentityStoreWithGithubAuth(t)

	// Create an entity and get its ID
	entityRegisterReq := &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "entity",
	}
	resp, err = is.HandleRequest(entityRegisterReq)
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("bad: resp: %#v, err: %v", resp, err)
	}
	entityID1 := resp.Data["id"].(string)

	// Create another entity and get its ID
	resp, err = is.HandleRequest(entityRegisterReq)
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("bad: resp: %#v, err: %v", resp, err)
	}
	entityID2 := resp.Data["id"].(string)

	// Create a group with the above created 2 entities as its members
	groupData := map[string]interface{}{
		"policies":          "testpolicy1,testpolicy2",
		"metadata":          []string{"testkey1=testvalue1", "testkey2=testvalue2"},
		"member_entity_ids": []string{entityID1, entityID2},
	}

	// Create a group and get its ID
	groupRegisterReq := &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "group",
		Data:      groupData,
	}
	resp, err = is.HandleRequest(groupRegisterReq)
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("bad: resp: %#v, err: %v", resp, err)
	}
	memberGroupID1 := resp.Data["id"].(string)

	// Create another group and get its ID
	resp, err = is.HandleRequest(groupRegisterReq)
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("bad: resp: %#v, err: %v", resp, err)
	}
	memberGroupID2 := resp.Data["id"].(string)

	// Create a group with the above 2 groups as its members
	groupData["member_group_ids"] = []string{memberGroupID1, memberGroupID2}
	resp, err = is.HandleRequest(groupRegisterReq)
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("bad: resp: %#v, err: %v", resp, err)
	}
	groupID := resp.Data["id"].(string)

	// Read the group using its name and check if all the fields are properly
	// set
	groupReq := &logical.Request{
		Operation: logical.ReadOperation,
		Path:      "group/id/" + groupID,
	}
	resp, err = is.HandleRequest(groupReq)
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("bad: resp: %#v, err: %v", resp, err)
	}

	expectedData := map[string]interface{}{
		"policies": []string{"testpolicy1", "testpolicy2"},
		"metadata": map[string]string{
			"testkey1": "testvalue1",
			"testkey2": "testvalue2",
		},
	}
	expectedData["id"] = resp.Data["id"]
	expectedData["name"] = resp.Data["name"]
	expectedData["member_group_ids"] = resp.Data["member_group_ids"]
	expectedData["member_entity_ids"] = resp.Data["member_entity_ids"]
	expectedData["creation_time"] = resp.Data["creation_time"]
	expectedData["last_update_time"] = resp.Data["last_update_time"]
	expectedData["modify_index"] = resp.Data["modify_index"]

	if !reflect.DeepEqual(expectedData, resp.Data) {
		t.Fatalf("bad: group data;\nexpected: %#v\n actual: %#v\n", expectedData, resp.Data)
	}

	// Update the policies and metadata in the group
	groupReq.Operation = logical.UpdateOperation
	groupReq.Data = groupData
	groupData["policies"] = "updatedpolicy1,updatedpolicy2"
	groupData["metadata"] = []string{"updatedkey=updatedvalue"}
	resp, err = is.HandleRequest(groupReq)
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("bad: resp: %#v, err: %v", resp, err)
	}

	// Check if updates are reflected
	groupReq.Operation = logical.ReadOperation
	resp, err = is.HandleRequest(groupReq)
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("bad: resp: %#v, err: %v", resp, err)
	}

	expectedData["policies"] = []string{"updatedpolicy1", "updatedpolicy2"}
	expectedData["metadata"] = map[string]string{
		"updatedkey": "updatedvalue",
	}
	expectedData["last_update_time"] = resp.Data["last_update_time"]
	expectedData["modify_index"] = resp.Data["modify_index"]
	if !reflect.DeepEqual(expectedData, resp.Data) {
		t.Fatalf("bad: group data; expected: %#v\n actual: %#v\n", expectedData, resp.Data)
	}

	// Check if delete is working properly
	groupReq.Operation = logical.DeleteOperation
	resp, err = is.HandleRequest(groupReq)
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("bad: resp: %#v, err: %v", resp, err)
	}

	groupReq.Operation = logical.ReadOperation
	resp, err = is.HandleRequest(groupReq)
	if err != nil {
		t.Fatal(err)
	}
	if resp != nil {
		t.Fatalf("expected a nil response")
	}
}

/*
Test groups hierarchy:
               eng
       |                |
     vault             ops
     |   |            |   |
   kube identity  build  deploy
*/
func TestIdentityStore_GroupHierarchyCases(t *testing.T) {
	var resp *logical.Response
	var err error
	is, _, _ := testIdentityStoreWithGithubAuth(t)
	groupRegisterReq := &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "group",
	}

	// Create 'kube' group
	kubeGroupData := map[string]interface{}{
		"name":     "kube",
		"policies": "kubepolicy",
	}
	groupRegisterReq.Data = kubeGroupData
	resp, err = is.HandleRequest(groupRegisterReq)
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("bad: resp: %#v, err: %v", resp, err)
	}
	kubeGroupID := resp.Data["id"].(string)

	// Create 'identity' group
	identityGroupData := map[string]interface{}{
		"name":     "identity",
		"policies": "identitypolicy",
	}
	groupRegisterReq.Data = identityGroupData
	resp, err = is.HandleRequest(groupRegisterReq)
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("bad: resp: %#v, err: %v", resp, err)
	}
	identityGroupID := resp.Data["id"].(string)

	// Create 'build' group
	buildGroupData := map[string]interface{}{
		"name":     "build",
		"policies": "buildpolicy",
	}
	groupRegisterReq.Data = buildGroupData
	resp, err = is.HandleRequest(groupRegisterReq)
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("bad: resp: %#v, err: %v", resp, err)
	}
	buildGroupID := resp.Data["id"].(string)

	// Create 'deploy' group
	deployGroupData := map[string]interface{}{
		"name":     "deploy",
		"policies": "deploypolicy",
	}
	groupRegisterReq.Data = deployGroupData
	resp, err = is.HandleRequest(groupRegisterReq)
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("bad: resp: %#v, err: %v", resp, err)
	}
	deployGroupID := resp.Data["id"].(string)

	// Create 'vault' with 'kube' and 'identity' as member groups
	vaultMemberGroupIDs := []string{kubeGroupID, identityGroupID}
	vaultGroupData := map[string]interface{}{
		"name":             "vault",
		"policies":         "vaultpolicy",
		"member_group_ids": vaultMemberGroupIDs,
	}
	groupRegisterReq.Data = vaultGroupData
	resp, err = is.HandleRequest(groupRegisterReq)
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("bad: resp: %#v, err: %v", resp, err)
	}
	vaultGroupID := resp.Data["id"].(string)

	// Create 'ops' group with 'build' and 'deploy' as member groups
	opsMemberGroupIDs := []string{buildGroupID, deployGroupID}
	opsGroupData := map[string]interface{}{
		"name":             "ops",
		"policies":         "opspolicy",
		"member_group_ids": opsMemberGroupIDs,
	}
	groupRegisterReq.Data = opsGroupData
	resp, err = is.HandleRequest(groupRegisterReq)
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("bad: resp: %#v, err: %v", resp, err)
	}
	opsGroupID := resp.Data["id"].(string)

	// Create 'eng' group with 'vault' and 'ops' as member groups
	engMemberGroupIDs := []string{vaultGroupID, opsGroupID}
	engGroupData := map[string]interface{}{
		"name":             "eng",
		"policies":         "engpolicy",
		"member_group_ids": engMemberGroupIDs,
	}

	groupRegisterReq.Data = engGroupData
	resp, err = is.HandleRequest(groupRegisterReq)
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("bad: resp: %#v, err: %v", resp, err)
	}
	engGroupID := resp.Data["id"].(string)

	/*
		fmt.Printf("engGroupID: %#v\n", engGroupID)
		fmt.Printf("vaultGroupID: %#v\n", vaultGroupID)
		fmt.Printf("opsGroupID: %#v\n", opsGroupID)
		fmt.Printf("kubeGroupID: %#v\n", kubeGroupID)
		fmt.Printf("identityGroupID: %#v\n", identityGroupID)
		fmt.Printf("buildGroupID: %#v\n", buildGroupID)
		fmt.Printf("deployGroupID: %#v\n", deployGroupID)
	*/

	var memberGroupIDs []string
	// Fetch 'eng' group
	engGroup, err := is.memDBGroupByID(engGroupID, false)
	if err != nil {
		t.Fatal(err)
	}
	memberGroupIDs, err = is.memberGroupIDsByID(engGroup.ID)
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(memberGroupIDs)
	sort.Strings(engMemberGroupIDs)
	if !reflect.DeepEqual(engMemberGroupIDs, memberGroupIDs) {
		t.Fatalf("bad: group membership IDs; expected: %#v\n actual: %#v\n", engMemberGroupIDs, memberGroupIDs)
	}

	vaultGroup, err := is.memDBGroupByID(vaultGroupID, false)
	if err != nil {
		t.Fatal(err)
	}
	memberGroupIDs, err = is.memberGroupIDsByID(vaultGroup.ID)
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(memberGroupIDs)
	sort.Strings(vaultMemberGroupIDs)
	if !reflect.DeepEqual(vaultMemberGroupIDs, memberGroupIDs) {
		t.Fatalf("bad: group membership IDs; expected: %#v\n actual: %#v\n", vaultMemberGroupIDs, memberGroupIDs)
	}

	opsGroup, err := is.memDBGroupByID(opsGroupID, false)
	if err != nil {
		t.Fatal(err)
	}
	memberGroupIDs, err = is.memberGroupIDsByID(opsGroup.ID)
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(memberGroupIDs)
	sort.Strings(opsMemberGroupIDs)
	if !reflect.DeepEqual(opsMemberGroupIDs, memberGroupIDs) {
		t.Fatalf("bad: group membership IDs; expected: %#v\n actual: %#v\n", opsMemberGroupIDs, memberGroupIDs)
	}

	groupUpdateReq := &logical.Request{
		Operation: logical.UpdateOperation,
	}

	// Adding 'engGroupID' under 'kubeGroupID' should fail
	groupUpdateReq.Path = "group/name/kube"
	groupUpdateReq.Data = kubeGroupData
	kubeGroupData["member_group_ids"] = []string{engGroupID}
	resp, err = is.HandleRequest(groupUpdateReq)
	if err == nil {
		t.Fatalf("expected an error response")
	}

	// Create an entity ID
	entityRegisterReq := &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "entity",
	}
	resp, err = is.HandleRequest(entityRegisterReq)
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("bad: resp: %#v, err: %v", resp, err)
	}
	entityID1 := resp.Data["id"].(string)

	// Add the entity as a member of 'kube' group
	entityIDReq := &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "group/id/" + kubeGroupID,
		Data: map[string]interface{}{
			"member_entity_ids": []string{entityID1},
		},
	}
	resp, err = is.HandleRequest(entityIDReq)
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("bad: resp: %#v, err: %v", resp, err)
	}

	// Create a second entity ID
	resp, err = is.HandleRequest(entityRegisterReq)
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("bad: resp: %#v, err: %v", resp, err)
	}
	entityID2 := resp.Data["id"].(string)

	// Add the entity as a member of 'ops' group
	entityIDReq.Path = "group/id/" + opsGroupID
	entityIDReq.Data = map[string]interface{}{
		"member_entity_ids": []string{entityID2},
	}
	resp, err = is.HandleRequest(entityIDReq)
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("bad: resp: %#v, err: %v", resp, err)
	}

	// Create a third entity ID
	resp, err = is.HandleRequest(entityRegisterReq)
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("bad: resp: %#v, err: %v", resp, err)
	}
	entityID3 := resp.Data["id"].(string)

	// Add the entity as a member of 'eng' group
	entityIDReq.Path = "group/id/" + engGroupID
	entityIDReq.Data = map[string]interface{}{
		"member_entity_ids": []string{entityID3},
	}
	resp, err = is.HandleRequest(entityIDReq)
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("bad: resp: %#v, err: %v", resp, err)
	}

	policies, err := is.groupPoliciesByEntityID(entityID1)
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(policies)
	expected := []string{"kubepolicy", "vaultpolicy", "engpolicy"}
	sort.Strings(expected)
	if !reflect.DeepEqual(expected, policies) {
		t.Fatalf("bad: policies; expected: %#v\nactual:%#v", expected, policies)
	}

	policies, err = is.groupPoliciesByEntityID(entityID2)
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(policies)
	expected = []string{"opspolicy", "engpolicy"}
	sort.Strings(expected)
	if !reflect.DeepEqual(expected, policies) {
		t.Fatalf("bad: policies; expected: %#v\nactual:%#v", expected, policies)
	}

	policies, err = is.groupPoliciesByEntityID(entityID3)
	if err != nil {
		t.Fatal(err)
	}

	if len(policies) != 1 && policies[0] != "engpolicy" {
		t.Fatalf("bad: policies; expected: 'engpolicy'\nactual:%#v", policies)
	}

	groups, err := is.transitiveGroupsByEntityID(entityID1)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 3 {
		t.Fatalf("bad: length of groups; expected: 3, actual: %d", len(groups))
	}

	groups, err = is.transitiveGroupsByEntityID(entityID2)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 2 {
		t.Fatalf("bad: length of groups; expected: 2, actual: %d", len(groups))
	}

	groups, err = is.transitiveGroupsByEntityID(entityID3)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 {
		t.Fatalf("bad: length of groups; expected: 1, actual: %d", len(groups))
	}
}
