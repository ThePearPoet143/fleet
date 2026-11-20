# Working in fleet

fleet sets up and checks the machines AI agents run on, on any system, from one Go program. Anyone can use it with
their own settings; this repo holds no one's personal details.

- ci's rules apply (`ci/agents/rules.md`): one Linear issue, one branch, one PR into `dev`, checked by ci's core.
- No personal details in the repo: no names, machine names, addresses, app IDs or keys. Each person's values live in
  their own settings file, outside the repo, written by `fleet setup`.
- A key is never typed, printed, logged or stored in settings; settings hold only its path.
- Every check the doctor runs is a function with tests fed sample output; a fix is a failing test first.
- Detect what you can (the system, installed tools); ask only the rest, and every question has a flag.
- Run every change on a real host before asking for review.
