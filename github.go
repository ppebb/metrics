package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type RepoResponse struct {
	Full_Name string
	Fork      bool
}

func github_get_account_repos(account string, org bool, token string) []RepoResponse {
	ret := []RepoResponse{}

	shouldContinue := true
	page := 1

	for shouldContinue {
		var response *http.Response

		if org {
			response = github_get_org_repos(account, token, page)
		} else {
			response = github_get_user_repos(account, token, page)
		}

		link := response.Header.Get("link")
		if len(link) == 0 || !strings.Contains(link, "rel=\"next\"") {
			shouldContinue = false
		}

		responses := []RepoResponse{}
		err := json.NewDecoder(response.Body).Decode(&responses)
		check(err)

		ret = append(ret, responses...)
		page++
	}

	return ret
}

func github_get_user_repos(username string, token string, page int) *http.Response {
	var endpoint string
	if len(token) > 0 {
		endpoint = "https://api.github.com/user/repos"
	} else {
		endpoint = fmt.Sprintf("https://api.github.com/users/%s/repos", username)
	}

	client := http.Client{}
	request, err := http.NewRequest(
		"GET",
		fmt.Sprintf(endpoint+"?per_page=100&page=%d",
			page,
		),
		nil,
	)
	check(err)

	if len(token) > 0 {
		request.Header.Set("Authorization", "token "+token)
	}

	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	response, err := client.Do(request)
	check(err)

	return response
}

func github_get_org_repos(org string, token string, page int) *http.Response {
	client := http.Client{}
	request, err := http.NewRequest(
		"GET",
		fmt.Sprintf("https://api.github.com/orgs/%s/repos?per_page=100&page=%d",
			org,
			page,
		),
		nil,
	)
	check(err)

	if len(token) > 0 {
		request.Header.Set("Authorization", "token "+token)
	}

	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	response, err := client.Do(request)
	check(err)

	return response
}
