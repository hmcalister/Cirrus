# Literal Cloud Service

The complete rewrite of the Literal Cloud Service — a daily server of cloud images. This project manages the database (storing users, roles), web frontend (for signups, account management, admin dashboard), and backend (handling account creation/deletion, managing auth with OAuth2, getting images from a repository, and sending emails).

### TODO

- Define database schema
    - Users
    - Daily Cloud information
    - Healthcheck (stored elsewhere?)
    - Trigger for cloud sending
- Set up backend
    - Repository pattern
        - ImageRepository
            - GetImage
        - EmailSenderRepository
            - SendEmail
    - Strategy Pattern
        - For authentication method to produce auth tokes
        - Use https://github.com/aidantwoods/go-paseto for token
        - Use https://github.com/markbates/goth for OAuth2
    - Account management
        - Create account
            - Authentication, Oauth
        - Delete account
            - GDPR compliance, complete deletion
- Email Server
    - Send emails... self-host or provider?
- Web frontend
    - HTMX?
    - Landing page
    - Signup page
    - (Auth: User) Account management / stats?
    - (Auth: Admin) Dashboard, stats