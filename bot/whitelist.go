package bot

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/companyzero/bisonrelay/zkidentity"
)

func (b *Bot) IsWhitelisted(id zkidentity.ShortID) bool {
	b.wlMtx.Lock()
	_, exists := b.wl[id.String()]
	b.wlMtx.Unlock()

	return exists
}

func (b *Bot) WhitelistAdd(id zkidentity.ShortID) error {
	defer b.wlMtx.Unlock()
	b.wlMtx.Lock()

	if _, exists := b.wl[id.String()]; exists {
		return fmt.Errorf("user is already whitelisted")
	}

	b.wl[id.String()] = time.Now().Unix()
	raw, err := json.Marshal(b.wl)
	if err != nil {
		return err
	}
	return os.WriteFile(b.wlFile, raw, 0o600)
}

func (b *Bot) WhitelistEntries() []string {
	defer b.wlMtx.Unlock()
	b.wlMtx.Lock()

	e := make([]string, 0, len(b.wl))
	for pubkey := range b.wl {
		e = append(e, pubkey)
	}
	return e
}

func (b *Bot) WhitelistRemove(id zkidentity.ShortID) error {
	defer b.wlMtx.Unlock()
	b.wlMtx.Lock()

	if _, exists := b.wl[id.String()]; !exists {
		return fmt.Errorf("user was not whitelisted")
	}

	delete(b.wl, id.String())
	raw, err := json.Marshal(b.wl)
	if err != nil {
		return err
	}
	return os.WriteFile(b.wlFile, raw, 0o600)
}
