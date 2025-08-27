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

### Community participants AKA Zarf Enthusiasts

Community participants engage with Zarf, 
contributing their time and energy in discussions or just generally helping out.  Additionally, community members participate in [Zarf community meetings](https://github.com/zarf-dev/zarf/issues/2613).

#### Responsibilities

- Keep participating!

#### Prerequisites

- Must follow the [OpenSSF Code of Conduct]
- Must follow the [Contribution Guide]

### Community Members

Community members are active **contributors** in the community. They help advance the Zarf project through commits, issues, and pull requests.
Members partipate through Github teams and pre-submit tests automatically run for their PRs.  Community members are expected to be active contributors in the community.

**Defined by:** Member of the Zarf GitHub "Community Member" team.

#### Responsibilities
- Can be assigned issues and pull request reviews
- Responsive to issues and pull request assigned to them
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
  - Filing or helping resolve an issue on GitHub
  - Contributing to a project, or community discussions (e.g. meetings, Slack,
    email discussion forums, Stack Overflow)

#### Promotion process

- Sponsored by 1 or more maintainers. **Note the following requirements for sponsors**:
  - Sponsors must have interactions with the prospective Member – e.g. 
    code/design/proposal review, coordinating on issues, etc.
- Open an issue in the project's repository
  - Ensure your sponsors are `@mentioned`
  - Describe and/or link to all your relevant contributions to the project
  - Sponsoring reviewers must comment on the issue/PR confirming their sponsorship

### Reviewers

Reviewers are trusted members of the Zarf community that are able to review changes to Zarf and indicate if those changes are ready for merge.
Reviewers have a strong and active track record of contribution to the Zarf project.

**Defined by:** Member of the Zarf GitHub ["Reviewer" team](https://github.com/orgs/zarf-dev/teams/reviewers).

#### Responsibilities

Commits to being an active contributor and reviewer as part of the Zarf project.  'Active' is defined as six PRs or PR reviews (or mix thereof) in **six months**, 
as measured by [Zarf Contributor Insights](https://github.com/zarf-dev/zarf/graphs/contributors) and [LFX Insights](https://insights.linuxfoundation.org/project/zarf/contributors?timeRange=alltime).
Is supportive of new and occasional contributors and helps get useful PRs in shape to commit.

#### Additional Privileges

Has GitHub or CI/CD rights to approve pull requests in specific directories

#### Prerequisites

Experience as a Community Member for at least 6 months
Is an Organization Member
Has reviewed, or helped review, at least 10 Pull Requests
Has analyzed and resolved test failures
Has demonstrated an in-depth knowledge of Zarf

#### Promotion process

- Nominated by a maintainer or may self-nominate
  - In case of self-nomination, sponsor must comment approval on the PR
- Sponsored by a maintainer, with no objections from other maintainers
- Completed by addition to Zarf Reviewer group


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

**Defined by:** Member of the Zarf GitHub ["Maintainer" team](https://github.com/orgs/zarf-dev/teams/maintainers).

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

#### Promotion process
- Sponsored by a maintainer or [Technical Steering Committee (TSC)](https://github.com/zarf-dev/zarf/blob/main/CONTRIBUTING.md#technical-steering-committee) members.
  - With no objections from other maintainers
  - With no objections from other TSC members.
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

However, should a Reviewer or above be deemed inactive for a significant period, any
Contributor or above may write an issue/PR requesting their removal from the ranks
(and `@mentioning` the inactive contributor in the hopes of drawing their attention).
The request must receive support (in comments) from a majority of Maintainers to proceed.


[OpenSSF Code of Conduct]: https://openssf.org/community/code-of-conduct/
[Contribution Guide]: ./CONTRIBUTING.md
