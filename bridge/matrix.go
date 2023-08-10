package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/decred/go-socks/socks"
	"github.com/decred/slog"
)

type MatrixClientConfig struct {
	DataDir  string
	User     string
	Password string
	Token    string
	Proxy    string
	Log      slog.Logger
}

type MatrixClient struct {
	cfg          MatrixClientConfig
	hc           *http.Client
	sincePath    string
	displayNames map[string]string
}

func NewMatrixClient(cfg MatrixClientConfig) *MatrixClient {
	var hc http.Client
	if cfg.Proxy != "" {
		proxy := &socks.Proxy{
			Addr:         cfg.Proxy,
			TorIsolation: true,
		}
		hc.Transport = &http.Transport{
			DialContext: proxy.DialContext,
		}
	}

	return &MatrixClient{
		cfg:          cfg,
		hc:           &hc,
		sincePath:    filepath.Join(cfg.DataDir, "since.json"),
		displayNames: make(map[string]string),
	}
}

func (m *MatrixClient) Join(ctx context.Context, room string) {
	apiURL := fmt.Sprintf("https://matrix.decred.org:8448/_matrix/client/v3/join/%s", room)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, nil)
	if err != nil {
		m.cfg.Log.Errorf("join: failed to create request: %v", err)
		return
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %v", m.cfg.Token))
	resp, err := m.hc.Do(req)
	if err != nil {
		m.cfg.Log.Errorf("join: request failed: %v", err)
		return
	}
	b, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		m.cfg.Log.Errorf("join: failed to read body: %v", err)
		return
	}
	if resp.StatusCode != 200 {
		m.cfg.Log.Errorf("failed to join room %q: %s", room, b)
		return
	}
	m.cfg.Log.Infof("joined %v", room)
}

func (m *MatrixClient) Part(ctx context.Context, room string) {
	apiURL := fmt.Sprintf("https://matrix.decred.org:8448/_matrix/client/v3/part/%s", room)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, nil)
	if err != nil {
		m.cfg.Log.Errorf("part: failed to create request: %v", err)
		return
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %v", m.cfg.Token))
	resp, err := m.hc.Do(req)
	if err != nil {
		m.cfg.Log.Errorf("part: request failed: %v", err)
		return
	}
	b, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		m.cfg.Log.Errorf("part: failed to read body: %v", err)
		return
	}
	if resp.StatusCode != 200 {
		m.cfg.Log.Errorf("failed to part room %q: %s", room, b)
		return
	}
	m.cfg.Log.Infof("parted %v", room)

}

type MsgReadReceipt struct {
	FullyRead string `json:"m.fully_read"`
	Read      string `json:"m.read"`
}

func (m *MatrixClient) SendReadReceipt(ctx context.Context, room, eventID string) {
	apiURL := fmt.Sprintf("https://matrix.decred.org:8448/_matrix/client/r0/rooms/%v/read_markers", room)

	rMsg := MsgReadReceipt{
		FullyRead: eventID,
		Read:      eventID,
	}
	msgData, err := json.Marshal(&rMsg)
	if err != nil {
		m.cfg.Log.Errorf("SendReadReceipt: failed to marshal: %v", err)
		return
	}
	r := bytes.NewReader(msgData)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, r)
	if err != nil {
		m.cfg.Log.Errorf("failed to create new request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %v", m.cfg.Token))
	resp, err := m.hc.Do(req)
	if err != nil {
		m.cfg.Log.Errorf("failed to send read receipt: %v", err)
		return
	}
	b, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		m.cfg.Log.Errorf("SendReadReceipt: failed to read body: %v", err)
		return
	}
	if resp.StatusCode != 200 {
		m.cfg.Log.Errorf("SendReadReceipt: %s %q", resp.Status, string(b))
		return
	}
	m.cfg.Log.Infof("sent read receipt with event id %v for room %v", eventID, room)
}

func (m *MatrixClient) SendMessage(ctx context.Context, room, msg string) error {

	var rb [32]byte
	_, err := rand.Read(rb[:])
	if err != nil {
		return err
	}
	txID := base64.URLEncoding.EncodeToString(rb[:])

	apiURL := fmt.Sprintf("https://matrix.decred.org:8448/_matrix/client/r0/rooms/%v/send/m.room.message/%v", room, txID)

	roomMsg := MsgRoomMsg{
		MsgType: "m.text",
		Body:    msg,
	}
	msgData, err := json.Marshal(&roomMsg)
	if err != nil {
		return err
	}
	r := bytes.NewReader(msgData)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, apiURL, r)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %v", m.cfg.Token))
	resp, err := m.hc.Do(req)
	if err != nil {
		return err
	}
	b, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		m.cfg.Log.Errorf("failed to read sendmessage response: %v", err)
		panic(err) // XXX
	}
	if resp.StatusCode != 200 {
		m.cfg.Log.Errorf("failed to sendmessage (statuscode:%v): %s", resp.StatusCode, b)
		return fmt.Errorf("failed to sendmessage (statuscode:%v): %s", resp.StatusCode, b)
	}
	m.cfg.Log.Infof("sendmessage room:%q msg:%q\n", room, msg)
	return nil
}

type UploadResponse struct {
	ContentURI string `json:"content_uri"`
}

func (m *MatrixClient) SendEmbed(ctx context.Context, room string, embed embeddedArgs) error {

	var err error
	mimeType := embed.typ
	var img image.Image
	var ext string
	switch mimeType {
	case "image/png":
		ext = ".png"
		img, err = png.Decode(bytes.NewReader(embed.data))
		if err != nil {
			return fmt.Errorf("unable to decode png: %v", err)
		}
	case "image/jpeg":
		ext = ".jpg"
		img, err = jpeg.Decode(bytes.NewReader(embed.data))
		if err != nil {
			return fmt.Errorf("unable to decode jpeg: %v", err)
		}
	case "image/gif":
		ext = ".gif"
		img, err = gif.Decode(bytes.NewReader(embed.data))
		if err != nil {
			return fmt.Errorf("unable to decode gif: %v", err)
		}
	default:
		return fmt.Errorf("unknown image type %q", mimeType)
	}
	bounds := img.Bounds()
	imgW := bounds.Max.X - bounds.Min.X
	imgH := bounds.Max.Y - bounds.Min.Y

	fname := strings.TrimSpace(escapeNick(embed.filename))
	if fname == "" {
		fname = "image_" + randomString() + ext
	}
	uploadURL := fmt.Sprintf("https://matrix.decred.org:8448/_matrix/media/r0/upload?filename=%s", fname)

	// Upload image.
	data := bytes.NewReader(embed.data)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, data)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %v", m.cfg.Token))
	req.Header.Set("Content-Type", mimeType)
	resp, err := m.hc.Do(req)
	if err != nil {
		return err
	}
	b, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return fmt.Errorf("failed to read upload response: %v", err)
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("failed to sendembed (statuscode:%v): %s", resp.StatusCode, b)
	}
	var upResponse UploadResponse
	if err := json.Unmarshal(b, &upResponse); err != nil {
		return fmt.Errorf("unable to unmarshal response: %v", err)
	}

	if upResponse.ContentURI == "" {
		return fmt.Errorf("content_uri on upload response is empty")
	}

	// Send image event.
	var rb [32]byte
	_, err = rand.Read(rb[:])
	if err != nil {
		return err
	}
	txID := base64.URLEncoding.EncodeToString(rb[:])

	apiURL := fmt.Sprintf("https://matrix.decred.org:8448/_matrix/client/r0/rooms/%v/send/m.room.message/%v", room, txID)

	roomMsg := MsgRoomMsg{
		MsgType: "m.image",
		Body:    fname,
		URL:     upResponse.ContentURI,
		Info: map[string]any{
			"w":        imgW,
			"h":        imgH,
			"size":     len(embed.data),
			"mimetype": mimeType,
		},
	}
	msgData, err := json.Marshal(&roomMsg)
	if err != nil {
		return err
	}
	r := bytes.NewReader(msgData)

	req, err = http.NewRequestWithContext(ctx, http.MethodPut, apiURL, r)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %v", m.cfg.Token))
	resp, err = m.hc.Do(req)
	if err != nil {
		return err
	}
	b, err = io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return fmt.Errorf("failed to read sendembed response: %v", err)
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("failed to sendembed (statuscode:%v): %s", resp.StatusCode, b)
	}

	m.cfg.Log.Infof("sendembed room:% size:%d fname:%q\n", room, len(embed.data), fname)
	return nil
}

type MsgLogin struct {
	Password string `json:"password"`
	Type     string `json:"type"`
	User     string `json:"user"`
}

type MsgLoginReply struct {
	UserID      string `json:"user_id"`
	AccessToken string `json:"access_token"`
	DeviceID    string `json:"device_id"`
	Homeserver  string `json:"home_server"`
	// Wellknown
}

func (m *MatrixClient) Login(ctx context.Context, user, pass string) string {
	const apiURL = "https://matrix.decred.org:8448/_matrix/client/v3/login"
	msg := MsgLogin{
		User:     user,
		Password: pass,
		Type:     "m.login.password",
	}
	msgData, err := json.Marshal(&msg)
	if err != nil {
		panic(err)
	}
	r := bytes.NewReader(msgData)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, r)
	if err != nil {
		m.cfg.Log.Errorf("login: failed to create request: %v", err)
		return ""
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := m.hc.Do(req)
	if err != nil {
		m.cfg.Log.Errorf("join: request failed: %v", err)
		return ""
	}

	b, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		panic(err)
	}
	var reply MsgLoginReply
	err = json.Unmarshal(b, &reply)
	if err != nil {
		panic(err)
	}
	m.cfg.Log.Infof("Login: %#v", reply)
	return reply.AccessToken
}

type ContentInfo struct {
	Size     int    `json:"size"`
	MimeType string `json:"mimetype"`
	W        int    `json:"w"`
	H        int    `json:"h"`
}

type Content struct {
	Body        string       `json:"body"`
	MsgType     string       `json:"msgtype"`
	Membership  string       `json:"membership"`
	DisplayName string       `json:"displayname"`
	MimeType    string       `json:"mimetype"`
	URL         string       `json:"url"`
	Info        *ContentInfo `json:"info"`
}

type MsgEventContent struct {
	AvatarURL        string    `json:"avatar_url"`
	DisplayName      string    `json:"displayname"`
	IsDirect         bool      `json:"is_direct"`
	JoinAuthorized   string    `json:"join_authorised_via_users_server"`
	Membership       string    `json:"membership"`
	Reason           string    `json:"reason"`
	ThirdPartyInvite MsgInvite `json:"third_party_invite"`
}

type MsgClientEventWithoutRoomID struct {
	Content        Content `json:"content"`
	EventID        string  `json:"event_id"`
	OriginServerTS int64   `json:"origin_server_ts"`
	Sender         string  `json:"sender"`
	// StateKey       string          `json:"state_key"`
	Type     string          `json:"type"`
	Unsigned MsgUnsignedData `json:"unsigned"`
}

type MsgInvite struct {
	DisplayName string `json:"display_name"`
}

type MsgRoomMsg struct {
	Body    string         `json:"body"`
	MsgType string         `json:"msgtype"`
	URL     string         `json:"url,omitempty"`
	Info    map[string]any `json:"info,omitempty"`
}

type MsgTimeline struct {
	Events    []MsgClientEventWithoutRoomID `json:"events"`
	Limited   bool                          `json:"limited"`
	PrevBatch string                        `json:"prev_batch"`
}

type MsgJoinedRoom struct {
	Timeline MsgTimeline `json:"timeline"`
}

type MsgSync struct {
	Filter      string `json:"filter"`
	FullState   bool   `json:"full_state"`
	SetPresence string `json:"set_presence"`
	Since       string `json:"since"`
	Timeout     int64  `json:"timeout"`
}

type MsgUnsignedData struct {
	Age int64 `json:"age"`
}

type MsgSyncReply struct {
	NextBatch string `json:"next_batch"`
	// Presence map[string]map[string]interface{} `json:"presence"`
	Rooms map[string]json.RawMessage `json:"rooms"`
	/*
	   DeviceLists map[string]interface{} `json:"device_lists"`
	   DeviceCount int64 `json:"device_one_time_keys_count"`
	   AccountData []Event `json:"account_data"`
	   Presence []Event `json:"presence"
	   Rooms json.RawMessage `json:"rooms"`
	   ToDevice string `json:"to_device"`
	*/
}

type MsgStatus struct {
	Presence string `json:"presence"`
}

func (m *MatrixClient) Status(ctx context.Context, status string) {
	apiURL := fmt.Sprintf("https://matrix.decred.org:8448/_matrix/client/r0/presence/%v/status", m.cfg.User)
	rMsg := MsgStatus{
		Presence: status,
	}
	msgData, err := json.Marshal(&rMsg)
	if err != nil {
		panic(err)
	}
	r := bytes.NewReader(msgData)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, apiURL, r)
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %v", m.cfg.Token))
	resp, err := m.hc.Do(req)
	if err != nil {
		panic(err)
	}
	b, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		panic(err)
	}
	if resp.StatusCode != 200 {
		m.cfg.Log.Errorf("Status: %s\n", b)
		return
	}
	m.cfg.Log.Infof("status set to %v", status)
}

type MsgDisplayName struct {
	DisplayName string `json:"displayname"`
}

func (m *MatrixClient) getDisplayName(ctx context.Context, nick string) (string, error) {
	name, exists := m.displayNames[nick]
	if exists {
		return name, nil
	}
	apiURL := fmt.Sprintf("https://matrix.decred.org:8448/_matrix/client/v3/profile/%s/displayname", nick)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %v", m.cfg.Token))
	resp, err := m.hc.Do(req)
	if err != nil {
		return "", err
	}
	b, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return "", err
	}
	if resp.StatusCode != 200 {
		m.cfg.Log.Errorf("getDisplayName status:%v %v", resp.StatusCode, string(b))
		return "", fmt.Errorf("getDisplayName status:%v %v", resp.StatusCode, string(b))
	}
	var d MsgDisplayName
	err = json.Unmarshal(b, &d)
	if err != nil {
		m.cfg.Log.Errorf("getDisplayName: failed to unmarshal: %v", err)
		return "", err
	}
	m.displayNames[nick] = d.DisplayName
	m.cfg.Log.Debugf("display name for %s: %s", nick, d.DisplayName)
	return d.DisplayName, nil
}

// encodeEmbeds returns a msg encoded for the BR network.
func (m *MatrixClient) encodeEmbeds(ctx context.Context, event *MsgClientEventWithoutRoomID) (string, error) {
	switch {
	case event.Content.MsgType == "m.image":
		// Download image.
		urlPrefix := "mxc://decred.org/"
		if !strings.HasPrefix(event.Content.URL, urlPrefix) {
			return "", fmt.Errorf("content does not have correct prefix url")
		}
		imgID := event.Content.URL[len(urlPrefix):]
		downloadURL := fmt.Sprintf("https://matrix.decred.org:8448/_matrix/media/r0/download/decred.org/%s", imgID)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
		if err != nil {
			return "", fmt.Errorf("unable to create http request: %v", err)
		}
		res, err := m.hc.Do(req)
		if err != nil {
			return "", fmt.Errorf("unable to download image to embed: %v", err)
		}
		rawData, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			return "", err
		}
		if res.StatusCode != http.StatusOK {
			return "", fmt.Errorf("not-ok status response downloading image to embed: %v", res.StatusCode)
		}

		if len(rawData) > 1024*1024 {
			return "", fmt.Errorf("image is too large to send on BR")
		}

		// Build embed string.
		mimeType := ""
		if event.Content.Info != nil {
			mimeType = event.Content.Info.MimeType
		}

		args := embeddedArgs{
			filename: event.Content.Body,
			typ:      mimeType,
			data:     rawData,
		}
		return args.String(), nil
	default:
		return event.Content.Body, nil
	}
}

func (m *MatrixClient) Sync(ctx context.Context, recvChan chan mtrxMsg) error {
	b, err := os.ReadFile(m.sincePath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	var since string
	if err == nil {
		err = json.Unmarshal(b, &since)
		if err != nil {
			return err
		}
	}

	r := regexp.MustCompile(`^\> \<@.*?\>`)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		apiURL := "https://matrix.decred.org:8448/_matrix/client/v3/sync"
		if since != "" {
			apiURL += fmt.Sprintf("?timeout=30000&since=%v", since)
		}
		m.cfg.Log.Debugf("%v", apiURL)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
		if err != nil {
			m.cfg.Log.Errorf("Sync: failed new request: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %v", m.cfg.Token))
		resp, err := m.hc.Do(req)
		if err != nil {
			m.cfg.Log.Errorf("Sync: failed to fetch: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}
		b, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return err
		}
		if resp.StatusCode != 200 {
			m.cfg.Log.Errorf("Sync: statuscode:%v %s", resp.StatusCode, b)
			time.Sleep(5 * time.Second)
			continue
		}

		var reply MsgSyncReply
		err = json.Unmarshal(b, &reply)
		if err != nil {
			return err
		}

		for key, v := range reply.Rooms {
			if key == "join" {
				m.cfg.Log.Debugf("%v %s", key, v)

				var s map[string]MsgJoinedRoom
				err = json.Unmarshal(v, &s)
				if err != nil {
					return err
				}
				for channel, v := range s {
					var bestTS int64
					var bestEvent string
					for _, event := range v.Timeline.Events {

						if event.OriginServerTS > bestTS {
							bestTS = event.OriginServerTS
							bestEvent = event.EventID
						}
						if event.Type == "m.room.message" && event.Sender != m.cfg.User {
							name, err := m.getDisplayName(ctx, event.Sender)
							if err != nil {
								m.cfg.Log.Errorf("sync: failed to get name: %v", err)
							}
							if name == "" {
								name = event.Sender
							}
							repl := r.FindString(event.Content.Body)
							if repl != "" {
								rawArgs := repl[3 : len(repl)-1]
								user, err := m.getDisplayName(ctx, rawArgs)
								if err != nil {
									m.cfg.Log.Errorf("sync: failed to get user from thread: %v", err)
								}
								if user != "" && user != rawArgs {
									event.Content.Body = r.ReplaceAllString(event.Content.Body, fmt.Sprintf("> <%v>", user))
								}
							}
							outMsg, err := m.encodeEmbeds(ctx, &event)
							if err != nil {
								m.cfg.Log.Errorf("unable to encode embed from matrix: %v", err)
							} else {
								newMsg := mtrxMsg{
									Network: networkMatrix,
									Nick:    name,
									Msg:     outMsg,
									Room:    channel,
								}
								recvChan <- newMsg
							}
						} else if event.Type == "m.room.member" && event.Sender != m.cfg.User && event.Content.Membership == "join" {
							m.displayNames[event.Sender] = event.Content.DisplayName
						}
						m.cfg.Log.Debugf("%v %v %v %v %#v", channel, event.EventID, event.Sender,
							event.Content.MsgType, strconv.QuoteToASCII(event.Content.Body))
					}
					if bestEvent != "" {
						m.SendReadReceipt(ctx, channel, bestEvent)
					}
				}
			}
		}
		nb, err := json.Marshal(reply.NextBatch)
		if err != nil {
			panic(err)
		}
		err = os.WriteFile(m.sincePath, nb, 0o600)
		if err != nil {
			panic(err)
		}
		since = reply.NextBatch
	}
}
func (m *MatrixClient) Run(ctx context.Context, recvChan chan mtrxMsg) error {
	return m.Sync(ctx, recvChan)
}
