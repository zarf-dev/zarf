# Zarf Contributor Ladder

This document outlines the various contributor roles in the Zarf project, along with their respective prerequisites and responsibilities.
It also defines the process by which users can request to change roles.  These roles are progressive, in that responsibilites and pre-requisites apply to subsequent rungs in the ladder.

- [Roles](#roles)
  - [Community Participants](#community-participants)
  - [Contributors](#contributors)
  - [Reviewers](#reviewers)
  - [Maintainers](#maintainers)
- [Inactive members](#inactive-members)

## Roles

### Community participants

Community participants engage with Zarf, 
contributing their time and energy in discussions or just generally helping out.  Additionally, community members participate in [Zarf community meetings](https://github.com/zarf-dev/zarf/issues/2613).

#### Responsibilities

- Keep participating!

#### Prerequisites

- Must follow the [OpenSSF Code of Conduct]
- Must follow the [Contribution Guide]

### Contributors

Contributors help advance the Zarf project through commits, issues, and pull requests.  Contributors participate through GitHub teams,
and pre-submit tests are automatically run for their PRs.

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

#### Prerequisites

- Enabled two-factor authentication on their GitHub account
- Have made contributions to the project in the form of:
  - Authoring or reviewing PRs on GitHub. At least one PR must be **merged**.
  - Filing or commenting on issues on GitHub
  - Contributing to a project, or community discussions (e.g. meetings, Slack,
    email discussion forums, Stack Overflow)
- Active contributor to Zarf

#### Promotion process `NEEDS REFINED?`

- Make at least one commit to a repository's code or open a pull request that gets merged into the repository

### Reviewers

Reviewers are trusted members of the Zarf community that are able to review changes to Zarf and indicate if those changes are ready for merge.  Reviewers have a strong and active track record of contribution to the Zarf project.

#### Responsibilities

Commits to being an active contributor and reviewer as part of the Zarf project.  'Active' is defined as six PRs or PR reviews (or mix thereof) in six months.
Is supportive of new and occasional contributors and helps get useful PRs in shape to commit.

#### Additional Privileges

Has GitHub or CI/CD rights to approve pull requests in specific directories

#### Prerequisites

Experience as a Contributor for at least 6 months
Is an Organization Member
Has reviewed, or helped review, at least 10 (`?`)Pull Requests
Has analyzed and resolved test failures
Has demonstrated an in-depth knowledge of Zarf

#### Promotion process

- Sponsored by a maintainer
  - With no objections from other maintainers
  - Done through PR to update the CODEOWNERS file, and addition to Zarf Maintainer group
- May self-nominate or be nominated by a maintainer
  - In case of self-nomination, sponsor must comment approval on the PR

### Maintainers

Maintainers are responsible for the project's overall health.
They are the only ones who can merge code contributions.
While code review is focused on code quality and correctness,
approval is focused on holistic acceptance of a contribution including:

- Backwards/forwards compatibility
- Adherence to API and style conventions
- Subtle performance and correctness issues
- Interactions with other parts of the system
- Consistency between code and documentation

#### Responsibilities

- Demonstrate sound technical judgment
- Maintain project quality control via code reviews
  - Focus on holistic acceptance of contribution
- Be responsive to review requests
- Mentor contributors and reviewers
- Approve and merge code contributions as appropriate
- Participate in OpenSSF or Zarf-specific community meetings, if possible
- Facilitating Zarf-specific community meetings, if possible

#### Additional Privileges

- Maintainer status may be a precondition to accepting especially large code contributions

#### Pre-requisites

- Reviewer for at least 1 month
- Reviewed at least 10 substantial PRs to the codebase
- Reviewed or got at least 30 PRs merged to the codebase

```or```

- Be a member of the Defense Unicorns or Radius Method organizations

#### Promotion process
- Sponsored by a maintainer
  - With no objections from other maintainers
  - Done through PR to update the CODEOWNERS file
- May self-nominate or be nominated by a maintainer
  - In case of self-nomination, sponsor must comment approval on the PR

Nominated by opening a PR against the Zarf repository, which adds their GitHub username to the OWNERS file for one or more directories.
Two maintainers must approve the PR.

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
Contributor or above may write an issue/PR requesting their removal from the ranks
(and `@mentioning` the inactive contributor in the hopes of drawing their attention).
The request must receive support (in comments) from a majority of Maintainers to proceed.


[OpenSSF Code of Conduct]: https://openssf.org/community/code-of-conduct/
[Contribution Guide]: ./CONTRIBUTING.md
