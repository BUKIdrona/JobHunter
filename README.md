# Job hunter

A personal Go program that looks for newly posted engineering roles every two hours, keeps the ones that fit Deepak Kumar Mandal's resume, and sends mail.

- If the posting lists a hiring address, it emails that address with the resume attached.
- If it does not, the role goes into one summary email to you, with the apply link.
- Roles already mailed are stored in `data/state.json` and are not sent again.

The skill match comes from the resume in this folder: Go and Fiber, C and C++, Python, SQL (PostgreSQL, MySQL, MongoDB), Docker, Kubernetes, Linux, REST APIs, and socket programming (TCP, UDP, SCTP). Titles have to be engineering roles in India, or remote roles that are not locked to another country.

## Sources

The program reads public job feeds and company career APIs. It does not log in to Indeed or Naukri, and it does not bypass their bot checks.

- Indeed RSS and Naukri's public search, using the queries in `config.yaml`
- [Remotive](https://remotive.com/api/remote-jobs), [Jobicy](https://jobicy.com/api/v2/remote-jobs), [Remote OK](https://remoteok.com/api), and [Arbeitnow](https://www.arbeitnow.com/api/job-board-api)
- Greenhouse, Lever, and Ashby boards listed in `config.yaml` (Groww, CRED, Meesho, GitLab, Cloudflare, and others)

A hiring email is included only when that address is printed on the public posting. Indeed and Naukri usually hide the recruiter address behind their own apply button, so those roles arrive as apply links unless the description itself lists an address.

Apply links point at the company page, or back at Remotive, Jobicy, or Remote OK when the role came from those feeds. Add another company by putting its board token in `config.yaml`.

## Run it

```bash
cp .env.example .env
```

Put a Gmail app password in `.env` as `SMTP_PASSWORD`. The account needs 2-step verification, then an app password from Google account settings. The username defaults to `deepakkumarmandal01092002@gmail.com`.

```bash
# one check, write the emails as files in data/outbox
make dry

# one real check
make once

# keep running, every 2 hours
make run
```

`make build` writes `bin/jobhunter`. A user systemd unit is in `deploy/jobhunter.service`.

Without `SMTP_PASSWORD` the program still runs, but it only writes `.eml` files and does not update state.

## Limits

Each run sends at most 8 hiring emails, and at most 20 per day. The summary email contains at most 25 roles. Hiring mail is sent from your own address, with your resume attached, and only to an address printed on that posting. A second copy of the process refuses to start, so two loops cannot mail the same role.
