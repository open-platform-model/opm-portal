// Package logs streams Pod logs to the change stream as bounded log topics.
//
// A log topic, "log:<namespace>/<pod>/<container>" or the same with a
// trailing "/previous", follows one container (0030:D10). Producer serves
// these topics to a stream.Broker, usually through a stream.Mux:
//
//   - The topic's read is get on the Pod's log subresource. The broker
//     authorizes it for the subscriber before the topic attaches, re-checks
//     it before every delivery through its send funnel, and again on every
//     reconnect.
//   - Admit then asks the read model whether an OPM inventory the
//     subscriber may read reaches the Pod. A Pod none reaches, one reached
//     only through objects the subscriber may not read, and a missing Pod
//     are refused alike, with the closing a missing permission gives.
//   - Activate opens one upstream stream per topic, shared by every
//     subscriber, as the reading identity after that identity's own check;
//     the release closes it.
//
// The portal bounds the output itself and never asks the API server for a
// byte limit, which would end a followed stream outright: an oversize line
// is cut and marked, lines over the rate are dropped and counted, and an
// oversize initial tail skips ahead with a marker. A container that stops
// ends its topic with a logend message.
//
// Log lines are untrusted text. They travel only as JSON strings in stream
// items and never appear in the portal's own logs or errors.
package logs
