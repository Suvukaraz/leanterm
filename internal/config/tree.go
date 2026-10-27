package config

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/Suvukaraz/leanterm/internal/secrets"
)

var (
	ErrNodeNotFound     = errors.New("node not found")
	ErrNodeProtected    = errors.New("node or descendant is protected")
	ErrInvalidParent    = errors.New("cannot move node into itself or its descendant")
	ErrRootProtected    = errors.New("root node cannot be edited or deleted")
)

type TreeNode struct {
	ID          string      `json:"id"`
	ParentID    string      `json:"parentId"`
	Name        string      `json:"name"`
	Title       string      `json:"title,omitempty"`
	IsGroup     bool        `json:"isGroup"`
	Method      string      `json:"method,omitempty"`
	Host        string      `json:"host,omitempty"`
	Port        int         `json:"port,omitempty"`
	Description string      `json:"description,omitempty"`
	Protected   bool        `json:"protected"`
	Favourite   bool        `json:"favourite"`
	HasPassword bool        `json:"hasPassword"`
	Children    []*TreeNode `json:"children,omitempty"`
}

type TreeManager struct {
	mu    sync.Mutex
	store *Store
	vault *secrets.Vault

	// Clipboard for copy/cut/paste
	clipboardIDs []string
	clipboardCut bool
}

func NewTreeManager(store *Store, vault *secrets.Vault) *TreeManager {
	return &TreeManager{
		store: store,
		vault: vault,
	}
}

func (tm *TreeManager) AddGroup(parentID, name string) (*Group, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	cfg := tm.store.cfg
	if parentID != "" && tm.isGroupProtectedLocked(parentID) {
		return nil, ErrNodeProtected
	}

	newID := uuid.NewString()
	grp := Group{
		ID:          newID,
		Parent:      parentID,
		Name:        name,
		Protected:   false,
		Screenshots: []string{},
	}

	cfg.Groups = append(cfg.Groups, grp)
	tm.store.MarkDirty()
	return &grp, nil
}

func (tm *TreeManager) AddConnection(parentID, name, title string) (*Connection, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	cfg := tm.store.cfg
	if parentID != "" && tm.isGroupProtectedLocked(parentID) {
		return nil, ErrNodeProtected
	}

	newID := uuid.NewString()
	if title == "" {
		title = name
		if cfg.General.AppendGroupNameToTitle && parentID != "" {
			if grp := tm.findGroupLocked(parentID); grp != nil {
				title = fmt.Sprintf("%s - %s", name, grp.Name)
			}
		}
	}

	conn := DefaultConnection(newID, parentID, name)
	conn.Title = title

	cfg.Connections = append(cfg.Connections, conn)
	tm.store.MarkDirty()
	return &conn, nil
}

func (tm *TreeManager) RenameNode(id, name, title string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if id == "" {
		return ErrRootProtected
	}

	if grp := tm.findGroupLocked(id); grp != nil {
		if grp.Protected {
			return ErrNodeProtected
		}
		grp.Name = name
		tm.store.MarkDirty()
		return nil
	}

	if conn := tm.findConnectionLocked(id); conn != nil {
		if conn.Protected {
			return ErrNodeProtected
		}
		conn.Name = name
		if title != "" {
			conn.Title = title
		}
		tm.store.MarkDirty()
		return nil
	}

	return ErrNodeNotFound
}

func (tm *TreeManager) SetProtected(ids []string, on bool) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	for _, id := range ids {
		if grp := tm.findGroupLocked(id); grp != nil {
			grp.Protected = on
		} else if conn := tm.findConnectionLocked(id); conn != nil {
			conn.Protected = on
		}
	}

	tm.store.MarkDirty()
	return nil
}

func (tm *TreeManager) SetFavourite(id string, on bool) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if conn := tm.findConnectionLocked(id); conn != nil {
		conn.Favourite = on
		tm.store.MarkDirty()
		return nil
	}
	return ErrNodeNotFound
}

func (tm *TreeManager) SetDescription(id, text string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if grp := tm.findGroupLocked(id); grp != nil {
		grp.Description = text
		tm.store.MarkDirty()
		return nil
	}
	if conn := tm.findConnectionLocked(id); conn != nil {
		conn.Description = text
		tm.store.MarkDirty()
		return nil
	}
	return ErrNodeNotFound
}

func (tm *TreeManager) DeleteNodes(ids []string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Check protection on any target node, ancestor, or descendant
	for _, id := range ids {
		if id == "" {
			return ErrRootProtected
		}
		if tm.isNodeOrAncestorOrDescendantProtectedLocked(id) {
			return ErrNodeProtected
		}
	}

	// Gather all IDs to delete (including descendants of groups)
	toDelete := make(map[string]bool)
	for _, id := range ids {
		tm.collectDescendantsLocked(id, toDelete)
	}

	// Delete from groups
	filteredGroups := make([]Group, 0, len(tm.store.cfg.Groups))
	for _, g := range tm.store.cfg.Groups {
		if !toDelete[g.ID] {
			filteredGroups = append(filteredGroups, g)
		}
	}
	tm.store.cfg.Groups = filteredGroups

	// Delete from connections and remove secrets
	filteredConns := make([]Connection, 0, len(tm.store.cfg.Connections))
	for _, c := range tm.store.cfg.Connections {
		if !toDelete[c.ID] {
			filteredConns = append(filteredConns, c)
		} else {
			if tm.vault != nil {
				tm.vault.DeleteConnection(c.ID)
			}
		}
	}
	tm.store.cfg.Connections = filteredConns

	tm.store.MarkDirty()
	return nil
}

func (tm *TreeManager) MoveNodes(ids []string, newParentID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if newParentID != "" && tm.findGroupLocked(newParentID) == nil {
		return ErrNodeNotFound
	}
	if newParentID != "" && tm.isGroupProtectedLocked(newParentID) {
		return ErrNodeProtected
	}

	for _, id := range ids {
		if id == "" {
			return ErrRootProtected
		}
		if tm.isNodeProtectedLocked(id) {
			return ErrNodeProtected
		}

		// Ensure newParent is not id or a descendant of id
		if tm.isDescendantOfLocked(newParentID, id) {
			return ErrInvalidParent
		}

		if grp := tm.findGroupLocked(id); grp != nil {
			grp.Parent = newParentID
		} else if conn := tm.findConnectionLocked(id); conn != nil {
			conn.Parent = newParentID
		}
	}

	tm.store.MarkDirty()
	return nil
}

func (tm *TreeManager) CloneNode(id string) (string, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	conn := tm.findConnectionLocked(id)
	if conn == nil {
		return "", ErrNodeNotFound
	}

	newID := uuid.NewString()
	cloned := *conn
	cloned.ID = newID
	cloned.Name = cloned.Name + " (Clone)"
	cloned.Title = cloned.Title + " (Clone)"

	// Assign new IDs for expect entries and variables
	if len(cloned.Expect) > 0 {
		newExpect := make([]ExpectEntry, len(cloned.Expect))
		for i, exp := range cloned.Expect {
			newExp := exp
			newExp.ID = fmt.Sprintf("e_%s", uuid.NewString()[:8])
			if tm.vault != nil && exp.Hidden {
				secret := tm.vault.GetConnection(conn.ID, "expect:"+exp.ID)
				tm.vault.SetConnection(newID, "expect:"+newExp.ID, secret)
			}
			newExpect[i] = newExp
		}
		cloned.Expect = newExpect
	}

	if len(cloned.Variables) > 0 {
		newVars := make([]UserVariable, len(cloned.Variables))
		for i, v := range cloned.Variables {
			newV := v
			newV.ID = fmt.Sprintf("v_%s", uuid.NewString()[:8])
			if tm.vault != nil && v.Hidden {
				secret := tm.vault.GetConnection(conn.ID, "variable:"+v.ID)
				tm.vault.SetConnection(newID, "variable:"+newV.ID, secret)
			}
			newVars[i] = newV
		}
		cloned.Variables = newVars
	}

	// Duplicate password / passphrase secrets
	if tm.vault != nil {
		tm.vault.DuplicateConnection(conn.ID, newID)
	}

	tm.store.cfg.Connections = append(tm.store.cfg.Connections, cloned)
	tm.store.MarkDirty()
	return newID, nil
}

func (tm *TreeManager) CopyNodes(ids []string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.clipboardIDs = append([]string(nil), ids...)
	tm.clipboardCut = false
}

func (tm *TreeManager) CutNodes(ids []string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	for _, id := range ids {
		if tm.isNodeOrAncestorOrDescendantProtectedLocked(id) {
			return ErrNodeProtected
		}
	}

	tm.clipboardIDs = append([]string(nil), ids...)
	tm.clipboardCut = true
	return nil
}

func (tm *TreeManager) PasteNodes(parentID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if len(tm.clipboardIDs) == 0 {
		return nil
	}

	if parentID != "" && tm.findGroupLocked(parentID) == nil {
		return ErrNodeNotFound
	}
	if parentID != "" && tm.isGroupProtectedLocked(parentID) {
		return ErrNodeProtected
	}

	if tm.clipboardCut {
		// Cut paste is a move
		for _, id := range tm.clipboardIDs {
			if tm.isDescendantOfLocked(parentID, id) {
				continue
			}
			if grp := tm.findGroupLocked(id); grp != nil {
				grp.Parent = parentID
			} else if conn := tm.findConnectionLocked(id); conn != nil {
				conn.Parent = parentID
			}
		}
		tm.clipboardIDs = nil
		tm.clipboardCut = false
	} else {
		// Copy paste: clone nodes into parentID
		for _, id := range tm.clipboardIDs {
			if conn := tm.findConnectionLocked(id); conn != nil {
				newID := uuid.NewString()
				cloned := *conn
				cloned.ID = newID
				cloned.Parent = parentID
				cloned.Name = cloned.Name + " (Copy)"
				if tm.vault != nil {
					tm.vault.DuplicateConnection(conn.ID, newID)
				}
				tm.store.cfg.Connections = append(tm.store.cfg.Connections, cloned)
			}
		}
	}

	tm.store.MarkDirty()
	return nil
}

func (tm *TreeManager) Search(query, field, fromID string, backwards bool) string {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if query == "" {
		return ""
	}

	q := strings.ToLower(query)
	var flatNodes []TreeNode
	tm.flattenTreeLocked("", &flatNodes)

	if len(flatNodes) == 0 {
		return ""
	}

	startIdx := 0
	if fromID != "" {
		for i, n := range flatNodes {
			if n.ID == fromID {
				if backwards {
					startIdx = i - 1
				} else {
					startIdx = i + 1
				}
				break
			}
		}
	}

	nLen := len(flatNodes)
	for step := 0; step < nLen; step++ {
		var idx int
		if backwards {
			idx = (startIdx - step + nLen) % nLen
		} else {
			idx = (startIdx + step) % nLen
		}

		node := flatNodes[idx]
		if tm.matchesQuery(node, q, field) {
			return node.ID
		}
	}

	return ""
}

func (tm *TreeManager) matchesQuery(n TreeNode, q, field string) bool {
	switch field {
	case "host":
		return strings.Contains(strings.ToLower(n.Host), q)
	case "desc":
		return strings.Contains(strings.ToLower(n.Description), q)
	case "name":
		fallthrough
	default:
		return strings.Contains(strings.ToLower(n.Name), q)
	}
}

func (tm *TreeManager) BuildTree() *TreeNode {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	root := &TreeNode{
		ID:       "",
		Name:     "My Connections",
		IsGroup:  true,
		Children: []*TreeNode{},
	}

	tm.populateChildrenLocked(root)
	return root
}

func (tm *TreeManager) populateChildrenLocked(parent *TreeNode) {
	sortGroupsFirst := tm.store.cfg.General.SortGroupsFirst

	var groups []*TreeNode
	for _, g := range tm.store.cfg.Groups {
		if g.Parent == parent.ID {
			node := &TreeNode{
				ID:          g.ID,
				ParentID:    g.Parent,
				Name:        g.Name,
				IsGroup:     true,
				Description: g.Description,
				Protected:   g.Protected,
				Children:    []*TreeNode{},
			}
			tm.populateChildrenLocked(node)
			groups = append(groups, node)
		}
	}

	var conns []*TreeNode
	for _, c := range tm.store.cfg.Connections {
		if c.Parent == parent.ID {
			hasPw := false
			if tm.vault != nil && tm.vault.GetConnection(c.ID, "password") != "" {
				hasPw = true
			}
			conns = append(conns, &TreeNode{
				ID:          c.ID,
				ParentID:    c.Parent,
				Name:        c.Name,
				Title:       c.Title,
				IsGroup:     false,
				Method:      c.Method,
				Host:        c.Host,
				Port:        c.Port,
				Description: c.Description,
				Protected:   c.Protected,
				Favourite:   c.Favourite,
				HasPassword: hasPw,
			})
		}
	}

	// Sort case-insensitive by name
	sort.Slice(groups, func(i, j int) bool {
		return strings.ToLower(groups[i].Name) < strings.ToLower(groups[j].Name)
	})
	sort.Slice(conns, func(i, j int) bool {
		return strings.ToLower(conns[i].Name) < strings.ToLower(conns[j].Name)
	})

	if sortGroupsFirst {
		parent.Children = append(groups, conns...)
	} else {
		all := append(groups, conns...)
		sort.Slice(all, func(i, j int) bool {
			return strings.ToLower(all[i].Name) < strings.ToLower(all[j].Name)
		})
		parent.Children = all
	}
}

func (tm *TreeManager) flattenTreeLocked(parentID string, out *[]TreeNode) {
	for _, g := range tm.store.cfg.Groups {
		if g.Parent == parentID {
			*out = append(*out, TreeNode{
				ID:          g.ID,
				ParentID:    g.Parent,
				Name:        g.Name,
				IsGroup:     true,
				Description: g.Description,
			})
			tm.flattenTreeLocked(g.ID, out)
		}
	}
	for _, c := range tm.store.cfg.Connections {
		if c.Parent == parentID {
			*out = append(*out, TreeNode{
				ID:          c.ID,
				ParentID:    c.Parent,
				Name:        c.Name,
				IsGroup:     false,
				Method:      c.Method,
				Host:        c.Host,
				Description: c.Description,
			})
		}
	}
}

func (tm *TreeManager) findGroupLocked(id string) *Group {
	for i := range tm.store.cfg.Groups {
		if tm.store.cfg.Groups[i].ID == id {
			return &tm.store.cfg.Groups[i]
		}
	}
	return nil
}

func (tm *TreeManager) findConnectionLocked(id string) *Connection {
	for i := range tm.store.cfg.Connections {
		if tm.store.cfg.Connections[i].ID == id {
			return &tm.store.cfg.Connections[i]
		}
	}
	return nil
}

func (tm *TreeManager) isNodeProtectedLocked(id string) bool {
	if grp := tm.findGroupLocked(id); grp != nil {
		return grp.Protected
	}
	if conn := tm.findConnectionLocked(id); conn != nil {
		return conn.Protected
	}
	return false
}

func (tm *TreeManager) isGroupProtectedLocked(id string) bool {
	if grp := tm.findGroupLocked(id); grp != nil {
		if grp.Protected {
			return true
		}
		if grp.Parent != "" {
			return tm.isGroupProtectedLocked(grp.Parent)
		}
	}
	return false
}

func (tm *TreeManager) isNodeOrAncestorOrDescendantProtectedLocked(id string) bool {
	if tm.isNodeProtectedLocked(id) {
		return true
	}
	// Check ancestors
	curr := id
	for curr != "" {
		if grp := tm.findGroupLocked(curr); grp != nil {
			if grp.Protected {
				return true
			}
			curr = grp.Parent
		} else if conn := tm.findConnectionLocked(curr); conn != nil {
			if conn.Protected {
				return true
			}
			curr = conn.Parent
		} else {
			break
		}
	}
	// Check descendants
	return tm.isDescendantProtectedLocked(id)
}

func (tm *TreeManager) isDescendantProtectedLocked(id string) bool {
	for _, g := range tm.store.cfg.Groups {
		if g.Parent == id {
			if g.Protected || tm.isDescendantProtectedLocked(g.ID) {
				return true
			}
		}
	}
	for _, c := range tm.store.cfg.Connections {
		if c.Parent == id && c.Protected {
			return true
		}
	}
	return false
}

func (tm *TreeManager) isDescendantOfLocked(childID, ancestorID string) bool {
	if childID == "" || ancestorID == "" {
		return false
	}
	if childID == ancestorID {
		return true
	}
	curr := childID
	for curr != "" {
		if curr == ancestorID {
			return true
		}
		if grp := tm.findGroupLocked(curr); grp != nil {
			curr = grp.Parent
		} else if conn := tm.findConnectionLocked(curr); conn != nil {
			curr = conn.Parent
		} else {
			break
		}
	}
	return false
}

func (tm *TreeManager) collectDescendantsLocked(id string, set map[string]bool) {
	set[id] = true
	for _, g := range tm.store.cfg.Groups {
		if g.Parent == id {
			tm.collectDescendantsLocked(g.ID, set)
		}
	}
	for _, c := range tm.store.cfg.Connections {
		if c.Parent == id {
			set[c.ID] = true
		}
	}
}
