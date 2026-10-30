// Package client is the Go client for Quorum, the approval engine.
//
// A Client talks to one Quorum server. Identity is attached per call with
// [Client.As], so one Client serves every maker and checker in a process:
//
//	q, err := client.New("https://quorum.example.com",
//		client.WithTenant("banking"))
//
//	req, err := q.As(client.Identity{UserID: "alice"}).CreateRequest(ctx, client.CreateRequest{
//		Type:    "wire_transfer",
//		Payload: transfer,
//	})
//
// Decisions arrive either by webhook, parsed and verified with
// [ParseWebhook], or by waiting on the request's event stream with
// [Client.WaitForDecision]. The types a Client exchanges are the server's own
// model types, so the client cannot drift from the API it talks to.
package client
