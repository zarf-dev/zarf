# Zarf contribution policy discussion draft

Status: Proposed for community discussion. These rules are not in effect until adopted. Automatic enforcement requires a separate implementation.

## Purpose

Zarf welcomes contributions from people who understand the problem they are addressing and take responsibility for their work. Maintainer review time is limited. Contributors must do the investigation, validation, and explanation needed to make their contributions ready for review.

This policy proposes contribution guardrails for the `zarf-dev` organization, starting with `zarf-dev/zarf`. It addresses submissions made without sufficient understanding or local validation, including unreviewed AI-generated issues and pull requests.

## Relationship to existing governance

Zarf already has governance and contribution rules:

- [CONTRIBUTING.md](https://github.com/zarf-dev/zarf/blob/main/CONTRIBUTING.md) defines the developer workflow, testing expectations, review requirements, and Technical Steering Committee.
- [CONTRIBUTOR_LADDER.md](https://github.com/zarf-dev/zarf/blob/main/CONTRIBUTOR_LADDER.md) defines Community Participants, Community Members, Reviewers, and Maintainers. The ladder was recently updated in [PR 5543](https://github.com/zarf-dev/zarf/pull/5543).
- [CHARTER.pdf](https://github.com/zarf-dev/zarf/blob/main/CHARTER.pdf) is the project's existing charter; adoption must respect its governance process.
- [CODE_OF_CONDUCT.md](https://github.com/zarf-dev/zarf/blob/main/CODE_OF_CONDUCT.md) applies the OpenSSF Code of Conduct to project spaces.
- [The PR template](https://github.com/zarf-dev/zarf/blob/main/.github/pull_request_template.md) requests a related issue and confirmation that the contributor workflow was followed.
- [Community issue 1](https://github.com/zarf-dev/community/issues/1) already proposes an AI contribution policy, including responsible use, human communication, disclosure, and treatment of good first issues.

This proposal supplements those rules. It does not change promotion criteria, merge authority, required approvals, commit verification, or sign-off requirements. The AI policy discussion should be incorporated into the same policy effort.

## Scope and contributor roles

For this proposal, an **outside contributor** is someone who is not a recognized Zarf Community Member, Reviewer, or Maintainer under the contributor ladder. The current ladder defines Community Members by membership in the `zarf-dev` GitHub organization, Reviewers by membership in `zarf-dev/reviewers`, and Maintainers by membership in `zarf-dev/maintainers`.

The mandatory PR entry requirements below apply to outside contributors. Recognized members continue to follow existing project standards and remain responsible for the quality of their submissions.

A previous merged PR, a GitHub contributor badge, or a self-described role does not grant an exemption.

These project roles are distinct from [GitHub repository permission roles](https://docs.github.com/en/organizations/managing-user-access-to-your-organizations-repositories/managing-repository-roles/repository-roles-for-an-organization), which are Read, Triage, Write, Maintain, and Admin.

## PR entry requirements for outside contributors

### Start with a triaged issue

An outside contributor may open a PR only after the associated issue has been triaged by an authorized project member and moved to **Ready** in the project's issue workflow.

The PR must link the issue and explain how the proposed changes address its agreed scope. An arbitrary issue reference does not satisfy this requirement. Opening an issue and immediately submitting a PR does not establish approval. Labels such as `good-first-issue` or `help-wanted` alone do not establish Ready status.

These requirements apply to draft PRs as well as PRs marked ready for review. Contributors should discuss scope and implementation questions on the issue before opening a PR. Ready status permits a submission; it does not promise review, acceptance, or a review deadline.

Under the proposed enforcement, a bot will immediately close outside-contributor PRs that have no linked issue or whose associated issue has not been triaged and moved to Ready. The closure notice will identify the unmet requirement and link to this policy. Contributors should correct the underlying issue and follow the stated reopening process rather than repeatedly opening replacement PRs.

### Include evidence of local testing

The PR description must include evidence that the contributor tested the submitted changes locally. A checked box, a statement that "tests pass," or successful hosted CI alone is insufficient.

Evidence must identify the tested revision, the commands run, the relevant environment, and the actual results. Include terminal output or attached logs showing completion and pass/fail summaries. Remove credentials and other sensitive information before attaching evidence. Refresh evidence when subsequent changes affect the behavior that was tested.

| Change | Required evidence |
| --- | --- |
| Any code change | Passing local unit tests and E2E tests, with commands and output. Include regression coverage for changed behavior where applicable. |
| Site or README changes | Screenshots of the updated rendered view. Before-and-after screenshots are preferred. Include relevant local documentation checks and build results. |
| Mixed code and documentation changes | Both sets of evidence. |

For the main Zarf repository, `make test-unit` runs the unit suite and `make test-e2e` runs the core CLI E2E suites. Other repositories must document their equivalent commands before adopting enforcement. The discussion below must settle whether focused E2E journeys can satisfy the requirement.

If required tests cannot be run locally, explain the limitation on the PR, however, the likelihood of excepting a PR that was not tested is very low. An explanation of missing tests does not itself satisfy the entry requirement.

## Responsible use of AI tools

Contributors remain accountable for every submitted line and every claim, regardless of the tools used. AI assistance does not replace understanding the code, reproducing a problem, running tests, or communicating directly with reviewers.

AI tools may assist a contribution when the contributor reviews, understands, and validates the result. Unverified generated submissions that transfer this work to maintainers may be closed without technical review.

## Maintainer discretion

Maintainers retain the right to close a PR when, in their judgment, it does not demonstrate sufficient understanding, care, relevance, or readiness for review. This includes submissions that appear to be unexamined AI output.

Maintainers do not need to prove that AI was used, establish a specific technical defect, conduct a full review, or provide a detailed justification before closing such a PR. Meeting the formal entry requirements does not create an entitlement to review or acceptance.

A brief policy reference may be used as the closure response. Maintainers are not required to enter repeated debate about the decision. All communication remains subject to the project's Code of Conduct.

## Issue guidelines

TODO: Define issue submission and triage requirements in a follow-up discussion. The existing [issue templates](https://github.com/zarf-dev/zarf/tree/main/.github/ISSUE_TEMPLATE) continue to apply.

Topics to settle include reproducible bug reports, supporting evidence for proposed features, duplicate searches, expectations for responding to clarification requests, and treatment of speculative or unverified generated reports. Filing an issue alone does not authorize a PR.

## Repeated violations

Initially, maintainers will observe repeated violations and record enough context to assess whether additional restrictions are needed. This proposal does not introduce automatic user blocking or a fixed violation threshold.

A future revision may define warnings, temporary restrictions, or blocking for repeated violations. That discussion must specify what counts as a repeat, who can impose restrictions, their duration, and any reconsideration process. Existing Code of Conduct enforcement remains in place.
