## ADDED Requirements

### Requirement: Challenge page displays notes below description
When a challenge has a non-null `notes` field, the challenge detail page SHALL render it below the description paragraph as a visually distinct instructional block (e.g., bordered aside or highlighted paragraph). When `notes` is null or empty, nothing is rendered in that position.

#### Scenario: Notes rendered when present
- **WHEN** a user navigates to `/challenges/:id` for a challenge whose `notes` field is non-null
- **THEN** the notes text is displayed below the description in a visually distinct block before the examples table

#### Scenario: Nothing rendered when notes is null
- **WHEN** a user navigates to `/challenges/:id` for a challenge whose `notes` field is null
- **THEN** no notes block is displayed on the page
