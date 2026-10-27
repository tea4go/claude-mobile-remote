# Beta review notes — replace the checklist with verified deployment details before submission

Claude Mobile Remote is a companion for a session hosted on the user's computer. In normal use, the user runs WhaleTerm or the standalone companion, then pairs the phone over a reachable LAN/VPN.

For review we provide an isolated **synthetic demo host** reachable via WSS. It uses the production pairing and transport protocol, but replies, tool approval and questions are scripted. It does not call Claude, execute shell commands or access personal projects. The demo does not demonstrate real AI reasoning or attachment interpretation. No account login is required.

1. Display the separately supplied private demo pairing QR on a second screen.
2. Open the app, choose Add pairing, grant camera access and scan it.
3. Confirm the connected PC is named “REVIEW DEMO - simulated Claude”.
4. Send a short message such as “Hello demo”.
5. A simulated tool approval appears. Choose Allow once or Deny. No command actually runs.
6. Answer the following scripted single/multiple choice questions and submit. A fixed response completes the turn.
7. Send another message; optionally use Stop while a decision is pending to test cancellation.

Multiple connected devices share this single demo. Do not enter private information. The QR expires; contact the supplied reviewer contact if it has expired. The supplementary real-use video demonstrates the actual PC/phone workflow.

Before submitting: verify mobile-data pairing and all steps above; supply the current private QR and expiry; provide an accessible, redacted video URL; remove this checklist. Do not submit this draft alone as working access instructions.
