package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/csrf"
	"golang.org/x/oauth2"
)

type OAuth struct {
	ProviderConfigs map[string]*oauth2.Config
}

func (oa OAuth) Connect(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	provider = strings.ToLower(provider)
	conf, ok := oa.ProviderConfigs[provider]
	if !ok {
		http.Error(w, "Unknown provider - invalid OAuth2 Service", http.StatusBadRequest)
		return
	}
	state := csrf.Token(r)
	setCookie(w, "oauth_state", state)
	url := conf.AuthCodeURL(state,
		oauth2.SetAuthURLParam("redirect_uri",
			redirectURI(r, provider)),
	)
	http.Redirect(w, r, url, http.StatusFound)
}

func (oa OAuth) Callback(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	provider = strings.ToLower(provider)
	conf, ok := oa.ProviderConfigs[provider]
	if !ok {
		http.Error(w, "Unknown provider - invalid OAuth2 Service", http.StatusBadRequest)
		return
	}

	state := r.FormValue("state")
	cookieState, err := readCookie(r, "oauth_state")
	if err != nil || cookieState != state {
		if err != nil {
			log.Println(err)
		}
		http.Error(w, "Invalid state parameter/ Invalid request", http.StatusBadRequest)
		return
	}
	deleteCookie(w, "oauth_state")

	code := r.FormValue("code")
	token, err := conf.Exchange(r.Context(), code, oauth2.SetAuthURLParam("redirect_uri", redirectURI(r, provider)))
	if err != nil {
		log.Println(err)
		http.Error(w, "Failed to exchange token", http.StatusInternalServerError)
		return
	}

	// w.Header().Set("Content-Type", "application/json")
	// enc := json.NewEncoder(w)
	// enc.SetIndent("", "  ")
	// enc.Encode(token)

	// json prettify

	client := conf.Client(r.Context(), token)
	resp, err := client.Post("https://api.dropboxapi.com/2/files/list_folder",
		"application/json",
		strings.NewReader(`{"path": ""}`),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	unpretty, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var pretty bytes.Buffer
	err = json.Indent(&pretty, unpretty, "", "  ")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	pretty.WriteTo(w)

	io.Copy(w, resp.Body)
}

func redirectURI(r *http.Request, provider string) string {
	if r.Host == "localhost:3000" {
		return fmt.Sprintf("http://localhost:3000/oauth/%s/callback", provider)
	}
	return fmt.Sprintf("https://fotohive.alextldr.com/oauth/%s/callback", provider)
}
