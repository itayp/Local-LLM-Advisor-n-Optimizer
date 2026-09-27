# Your data and Local LLM Advisor

**In short:** everything this app learns stays on your computer. It never
sends anything you type, any of your files, or how you use it to anyone.
There's no account, no tracking, no ads and no usage statistics.

It does need the internet for a few things you ask it to do: fetching its
list of models, fetching published test scores for those models, installing
Ollama, and checking for a newer version. The full list is below. Each of
those requests asks for public information. None of them sends anything
about you or your computer.

---

## What the app looks at on your computer

- **Your hardware:** the processor, how much memory you have, your graphics
  card and how much memory it has, your operating system and its version,
  the computer's name, and free disk space. The app gets this by asking the
  operating system. It never needs your password.
- **Ollama:** whether it's installed and running, which models it has
  downloaded, and which one is loaded right now.
- **Chat apps:** whether a few well-known chat apps (like LM Studio) are
  installed. It only checks whether they exist. It never opens them or
  reads anything inside them.
- **During a test you start:** how much memory, processor and graphics card
  the test uses, checked once a second.

It does **not** read your documents or photos, your browser, or any
conversation you've had in a chat app.

## What it keeps, and where

The app keeps what it learns in one folder. Only your user account can open
it:

| Your computer | The folder |
|---|---|
| Mac | `~/Library/Application Support/Advisor` |
| Windows | `%LOCALAPPDATA%\Advisor` |
| Linux | `~/.local/share/advisor` |

Inside that folder there's one database. It holds:

- a description of your computer (including its name), saved each time the
  app starts, so an old test result always says which hardware it came from;
- which models Ollama had installed;
- the model list and the published scores it downloaded;
- the results of every test you ran;
- your settings, and the log of the daily check for new models.

If the app starts Ollama for you, Ollama's own messages are saved in the
same folder. On Linux, if you let the app install Ollama, Ollama itself is
installed there too.

A few things live outside that folder, and only after you ask for them:

- **"Start at login"**, if you turn it on: a small login entry, which is the
  normal way each operating system does this.
- **On Windows:** the app's name, registered so Windows can show its
  notifications.
- **While Ollama is installing:** the installer, in your computer's temporary
  folder.

It does **not** keep the text a model writes during a test. It only keeps
the timings.

Your downloaded models belong to Ollama. They're stored in Ollama's own
folder, not the app's.

## What leaves your computer: the complete list

| When | Where it goes | What happens |
|---|---|---|
| You click **Fetch the model list** | Hugging Face (huggingface.co and its download servers) | The app downloads the file list for each model on its curated list, plus a small piece from the start of each model file (its description, never the model itself). |
| Right after that, in the background | Hugging Face, and Epoch AI (epoch.ai) | The app downloads the benchmark scores these sources publish about the models on its list. |
| Once a day, while **Check for new models** is on (Settings) | The same places | The same as above, to spot new models that suit you. It also looks at which models the makers on its list have published. This never starts until you've fetched the model list yourself at least once. |
| You open the Ollama setup screen and Ollama isn't installed | ollama.com and GitHub | The app asks how big the installer is, so the button can tell you. |
| You click **Install Ollama** | ollama.com and GitHub | The app downloads Ollama's official installer and checks it (see below). |
| You click **Download** on a model | Ollama's model library, through Ollama | The app gives Ollama the name of a model from its own curated list, and Ollama downloads it the way it normally does. |
| You click **Check for updates** | GitHub | The app asks what the newest version of this app is. |

**What goes with each request:** the address of what's being asked for,
plus the app's name and version (for example `local-llm-advisor/1.2.0`).
Like any website you visit, the server can see your internet address.
Nothing else is sent: no account, no cookies, no ID, nothing about your
computer, and nothing you've typed. Every request is encrypted (HTTPS).

The app can only reach the places on this list. That's built into the
program itself, not left as a setting, and it's checked every time the app
is built.

**Links you click in the app** (to a model's page, or to where a score was
published) open in your web browser, like any other link.

## What never leaves your computer

- **Anything you type.** There's nowhere in the app to type a prompt. When
  you run a test, the app sends its own built-in text (a short essay about a
  year on an allotment garden, the same on every computer) to Ollama on
  **your** computer, and nowhere else. The program is built so that it has
  no way to send any other text to a model.
- **Your computer's description, your test results and your settings.**
- **How you use the app.** There are no usage statistics, crash reports or
  analytics of any kind.

If Ollama is set up to run on a different computer, the app ignores that
setting and says so. It only ever works with Ollama on this computer.

## Who can reach the app

The app runs as a small program in the background, and its screens open in
your web browser at `http://127.0.0.1`. That address only exists inside your
own computer, so other computers on your network, and anyone on the
internet, can't reach it.

Websites you visit can't reach it either. The app only answers its own
screens: if another web page tries to send it a request, or to show it
inside that page, the app refuses. And the app's own screens can only talk
to the app. They can't send anything anywhere else.

The app doesn't ask for an administrator password and installs nothing
system-wide.

## Deleting everything

Open **Settings** and click **Delete everything this app has stored**. The
app shows you exactly what will go, and does nothing until you confirm. It
then:

- deletes the database (everything listed under "What it keeps");
- turns off "Start at login" if it was on, and on Windows removes the app's
  notification registration;
- removes any Ollama installer it left in your temporary folder, and the
  Ollama messages it saved;
- closes itself. If you open it again, it starts fresh.

Your downloaded models and Ollama stay installed. You can remove models one
by one on the **Models** screen. On Linux, if the app installed Ollama for
you, that copy of Ollama stays in the data folder, and the app tells you
where. Delete that folder to remove it.

Uninstalling the app does **not** remove the data folder, so a reinstall
picks up where you left off. If you want everything gone, click the button
before you uninstall, or delete the folder from the table above yourself
after quitting the app.

## Downloads you can trust

- **Ollama:** before the app opens or unpacks Ollama's installer, it checks
  that the file is exactly the one Ollama published. Ollama publishes a
  checksum (a fingerprint of the file) with every release, and the app
  compares the two. If they don't match, or Ollama didn't publish one, the
  app deletes the download, installs nothing, and tells you why. The
  download only ever comes from ollama.com and Ollama's own page on GitHub.
- **Models:** Ollama checks each model file it downloads against the
  fingerprint its library publishes for it.
- **This app:** the release page lists a SHA-256 checksum for every file
  on it. If the Mac or Windows version isn't signed yet, your computer
  shows a warning the first time you open it. `INSTALL.md` walks you
  through that.

## Found a problem?

If you think you've found a security or privacy problem, please report it
privately through the **Security** tab of the project's GitHub page (under
"Report a vulnerability"). If that isn't available, open an issue asking
for a private contact, and leave the details out of it.

---

### For the technically curious

- The complete list of internet addresses the app may contact is one file:
  `internal/egress/hosts.go`. Every connection the app makes goes through
  that package. A test (`internal/archtest`) fails the build if any other
  part of the program opens a connection of its own, turns off certificate
  checks, or sends a credential or cookie.
- The server listens on `127.0.0.1` only. That's a constant in the code,
  not a setting. It answers a browser only from its own origin, sends a
  Content-Security-Policy that limits its pages to itself
  (`connect-src 'self'`, `frame-ancestors 'none'`), and sends no CORS
  headers.
- What a model can be sent is a sealed type (`internal/suite`) that only the
  benchmark text embedded in the app can produce.
- The decisions behind all of this, and what was checked, are in
  `ARCHITECTURE.md` (D-64 to D-69) and `claude/step-12-security.md`.
