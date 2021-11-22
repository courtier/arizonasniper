package main

import (
	"time"

	"github.com/valyala/fasthttp"
)

func webhookNitro(delay string, nitroType, username, authorAvatar string) {
	if !config.CustomWebhookNitro || config.CustomWebhookLink == "" {
		return
	}

	link := config.CustomWebhookLink

	const color string = "16749250"

	body := `
	{
		"embeds": [
			{
				"title": "Sniped Nitro!",
				"color": ` + color + `,
				"timestamp": "` + time.Now().Format(time.RFC3339) + `",
				"fields": [
					{
						"name": "Type",
						"value": "` + "`" + nitroType + "`" + `",
						"inline": true
					},
					{
						"name": "Delay",
						"value": "` + "`" + delay + "s`" + `",
						"inline": true
					}
				],
		  		"author": {
					"name": "` + username + `",
					"icon_url": "` + authorAvatar + `"
				},
		  		"footer": {
					"text": "Arizona Sniper"
		  		}
			}
	 	],
		"username":  "Arizona"
	}
	`

	req := fasthttp.AcquireRequest()
	req.Header.SetContentType("application/json")
	req.SetBodyString(body)
	req.Header.SetMethod("POST")
	req.SetRequestURI(link)
	res := fasthttp.AcquireResponse()

	if err := fasthttp.Do(req, res); err != nil {
		return
	}

	fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(res)
}
