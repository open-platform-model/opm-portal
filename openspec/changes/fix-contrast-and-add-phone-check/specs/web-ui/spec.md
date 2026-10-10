## ADDED Requirements

### Requirement: Every page holds its width at a phone viewport

A browser test SHALL open every page of the F1 capture at a 360 CSS px wide viewport in Chromium,
Firefox and WebKit, and SHALL fail when a page's scroll width exceeds 360 px. The test SHALL prove
that it can fail by detecting an element it adds that is wider than the viewport. Pages that the
UI gains SHALL be added to the test's page list by the change that adds them.

#### Scenario: A page fits the phone viewport

- **WHEN** the test opens a page of the F1 capture at 360 px wide, in any of the three browsers
- **THEN** the page's scroll width is 360 px or less and the test passes

#### Scenario: A page scrolls sideways

- **WHEN** a page has content that makes its scroll width exceed 360 px
- **THEN** the test fails and names the page, the browser and the elements that reach past the
  viewport

#### Scenario: The check can fail

- **WHEN** the test adds a 500 px wide element to a page at the 360 px viewport
- **THEN** it measures a scroll width above 360 px in each browser, or the test fails

#### Scenario: The test runs where the other browser tests run

- **WHEN** `task test:browser` runs, locally or in the nightly `E2E` workflow
- **THEN** it includes `TestBrowserPhone`

### Requirement: Muted text meets WCAG 2.2 AA contrast in both themes

Text drawn with the muted ink token SHALL have a contrast ratio of at least 4.5:1 against each
surface token it is drawn on (page, card, secondary card and field), in the light and the dark
theme. The ratio is computed from the token hex values in `portal.css` with the WCAG relative
luminance formula and is not rounded.

#### Scenario: Muted text on the page and on cards

- **WHEN** the light theme draws muted text on the page background, on a card or on a secondary card
- **THEN** each pair has a ratio of at least 4.5:1

#### Scenario: A token pair falls below its floor

- **WHEN** a change lowers the ratio of a listed token pair under its floor in either theme
- **THEN** the contrast test fails and names the pair, the theme and the measured ratio

### Requirement: Text inputs and selects have a boundary of at least 3:1

The border of a text input or select SHALL use the control-border token, and the token SHALL have a
contrast ratio of at least 3:1 against the field fill and against each surface a field is drawn on,
in the light and the dark theme. Borders that only decorate a control that has a text label are not
held to this ratio.

#### Scenario: A filter field on its panel

- **WHEN** a filter input or select is drawn on the filter panel, in either theme
- **THEN** its border has a ratio of at least 3:1 against the field fill and against the panel

#### Scenario: The two dark blocks agree

- **WHEN** the dark theme tokens are set both under the OS preference and under the stored Dark
  choice
- **THEN** every token in the contrast test's pair list has the same value in both blocks, or the
  test fails

#### Scenario: A token the test cannot read

- **WHEN** a listed token holds a value that is not a six-digit hex colour
- **THEN** the contrast test fails and names the token instead of skipping the pair
