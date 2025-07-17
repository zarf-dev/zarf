# Zarf Contributor Ladder

This document outlines the various contributor roles in the Zarf project, along with their respective prerequisites and responsibilities.
It also defines the process by which users can request to change roles.  These roles are progressive, in that responsibilites and pre-requisites apply to subsequent rungs in the ladder.

- [Roles](#roles)
  - [Community Members](#community-members)
  - [Contributors](#contributors)
  - [Reviewers](#reviewers)
  - [Maintainers](#maintainers)
- [Inactive members](#inactive-members)

## Roles

### Community members

Community participants engage with Zarf, contributing their time and energy in discussions or just generally helping out.  Additionally, community members participate in [Zarf community meetings](https://github.com/zarf-dev/zarf/issues/2613).

#### Pre-requisites

- Must follow the [OpenSSF Code of Conduct]
- Must follow the [Contribution Guide]

#### Responsibilities

- Keep it up!

### Contributors

Contributors help advance the Zarf project through commits, issues, and pull requests.  Contributors participate through GitHub teams,
and pre-submit tests are automatically run for their PRs.

#### Pre-requisites

- Enabled two-factor authentication on their GitHub account
- Have made contributions to the project in the form of commits, issues, or pull requests.

#### Responsibilities
- Can be assigned issues and PRs
- Responsive to issues and PRs assigned to them
- Others can ask for reviews with a `/cc @username`.
- Responsive to mentions of teams they are members of
- Active owner of code they have contributed (unless ownership is explicitly transferred)
  - Ensures code is well tested and that tests consistently pass
  - Addresses bugs or issues discovered after code is accepted

#### Privileges

- Tests run against their PRs automatically

#### Promotion process

- Make at least one commit to a repository's code or open a pull request that gets merged into the repository

### Reviewers

#### Pre-requisites

#### Responsibilities

#### Privileges

#### Promotion process

### Maintainers

Maintainers are responsible for the project's overall health.
They are the only ones who can approve and merge code contributions.
While triage and code review is focused on code quality and correctness,
approval is focused on holistic acceptance of a contribution including:

- backwards/forwards compatibility
- adherence to API and style conventions
- subtle performance and correctness issues
- interactions with other parts of the system
- consistency between code and documentation

**Defined by:** "Maintain" permissions in the project and an entry in its CODEOWNERS file

#### Pre-requisites

- Triager for at least 1 month
- Reviewed at least 10 substantial PRs to the codebase
- Reviewed or got at least 30 PRs merged to the codebase

#### Responsibilities

- Demonstrate sound technical judgment
- Maintain project quality control via code reviews
  - Focus on holistic acceptance of contribution
- Be responsive to review requests
- Mentor contributors and triagers
- Approve and merge code contributions as appropriate
- Participate in OpenSSF or Scorecard-specific community meetings, if possible
- Facilitating Scorecard-specific community meetings, if possible and comfortable

#### Privileges

- Same as for Triager
- Maintainer status may be a precondition to accepting especially large code contributions

#### Promotion process
- Sponsored by a maintainer
  - With no objections from other maintainers
  - Done through PR to update the CODEOWNERS file
- May self-nominate or be nominated by a maintainer
  - In case of self-nomination, sponsor must comment approval on the PR

## Inactive members
A core principle in maintaining a healthy community is encouraging active participation.
It is inevitable that a contributor's focus will change over time
and there is no expectation they'll actively contribute forever.

Any contributor at any level described above may write an issue (or PR, if CODEOWNER changes are necessary)
asking to step down to a lighter-weight tier or to depart the project entirely.
Such requests will hopefully come after thoughtful conversations with the rest of the team
and with sufficient forewarning for the others to prepare. However, sometimes "life happens".
Therefore, the change in responsibilities will be understood to take immediate effect,
regardless of whether the issue/PR has been acknowledged or merged.

However, should a Triager or above be deemed inactive for a significant period, any
Community Member or above may write an issue/PR requesting their removal from the ranks
(and `@mentioning` the inactive contributor in the hopes of drawing their attention).
The request must receive support (in comments) from a majority of Maintainers to proceed.


[OpenSSF Code of Conduct]: https://openssf.org/community/code-of-conduct/
[Contribution Guide]: ./CONTRIBUTING.md
