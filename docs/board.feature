Feature: Build software through a prompt board
  Background:
    Given the local workspace is running in demo mode

  Scenario: Create a prompt ticket
    When I create a ticket with a title and a prompt
    Then it appears in Todo
    And it remains after refreshing the page

  Scenario: Save prompt edits
    Given I select a Todo ticket
    When I edit its prompt and save changes
    Then its saved prompt is shown when I select it again

  Scenario: Start a ticket by dragging
    Given a Todo ticket has a saved prompt
    When I drag it into Doing
    Then its saved prompt is queued exactly once
    And it is labelled Queued or Running
    And its prompt cannot be edited during the run
    When the demo agent completes successfully
    Then the ticket moves into Done
    And its response identifies the work as simulated
    And no repository files have been modified

  Scenario: Start a ticket without dragging
    Given I select a Todo ticket
    When I choose Run ticket
    Then the same queue workflow starts

  Scenario: Sequential queue
    Given one ticket is running
    When I start a second Todo ticket
    Then the second ticket waits in FIFO order
    And at most one agent executes at a time

  Scenario: Provider is not configured
    When I inspect the agent selector
    Then Codex and Claude are labelled Not connected
    And only the demo adapter can run
    When an API client requests an unconfigured provider
    Then the API rejects the run without queuing it

  Scenario: Paste a reference image
    Given I am editing a Todo ticket
    When I paste a PNG, JPEG or WebP image up to 5 MB
    Then a thumbnail appears under the prompt
    When I select the thumbnail
    Then I see a larger image in a dismissible dialog
    And I can remove the attachment before saving

  Scenario: Invalid attachment
    When I paste an unsupported or oversized image
    Then I see a useful validation message
    And the invalid image is not attached

  Scenario: Review changes
    Given I select a completed ticket
    Then I see a summary of simulated changed files
    When I select a file
    Then I see its unified diff with additions and removals distinguished

  Scenario: Find tickets
    When I search by title, prompt or ticket ID
    Then only matching tickets appear
    When I filter by status
    Then only tickets with that status appear

  Scenario: API failure
    Given the backend cannot be reached
    When I attempt to save or run a ticket
    Then I see an error
    And the UI does not claim the operation succeeded

  Scenario: Restart recovery
    Given a ticket was running when the backend stopped
    When the backend restarts
    Then the interrupted ticket is marked Failed
    And queued tickets resume in their existing order

Feature: Execute real coding agents in isolated workspaces
  Background:
    Given the selected agent CLI and server API credentials are configured
    And a Git repository with a saved HEAD is registered under an allowed server root

  Scenario: Real agent run
    Given a Codex or Claude ticket selects that repository
    And the ticket allows edits in its isolated worktree
    When I start the ticket
    Then the queued attempt preserves its prompt, images, permission and base commit
    And the agent runs in a detached worktree for that attempt
    And pasted images are delivered to the agent
    And the source checkout is unchanged
    And successful completion moves the ticket to Done
    And changed files show actual Git diffs including staged and untracked changes

  Scenario: Live activity
    Given a ticket is running
    Then server-sent events update run activity and board status
    And event IDs allow replay after a stream reconnect
    And polling remains available if streaming is unavailable

  Scenario: Cancel an attempt
    Given a ticket is queued or running
    When I cancel the run
    Then the ticket becomes Cancelled
    And an active agent process group is terminated
    And partial changes remain in the retained worktree for review

  Scenario: Timeout or failed tests
    Given a ticket is running
    When its execution deadline expires or its configured tests fail
    Then the attempt is Failed
    And it never moves into Done
    And its error and partial changes are saved

  Scenario: Retry with history
    Given a ticket is Failed or Cancelled
    When I retry it
    Then a new attempt starts in a new worktree from the repository's current saved HEAD
    And previous prompts, events, responses and diffs remain reviewable

  Scenario: Restrict workspace access
    When I register a path outside the configured roots or a non-Git directory
    Then the API rejects it
    And no agent process starts

  Scenario: Read-only execution
    Given I choose read-only permission
    When the agent runs
    Then its CLI is configured for read-only access
    And an attempt that nevertheless changes tracked files is Failed

  Scenario: Trusted test commands
    Given the repository has a Vitest or Pytest preset and relative test directory
    And the ticket explicitly enables running repository tests
    When the agent finishes
    Then the preset runs without a shell command string
    And provider API keys are excluded from the test process environment
    And test output and outcome are saved in run activity

  Scenario: Server-only credentials
    When I inspect repository settings
    Then I see provider readiness and the required server configuration
    And no API keys are returned to the browser

Feature: Organise tickets into groups
  Scenario: Create a group and a grouped ticket
    When I add a named group with a colour
    Then the group appears in Todo even when empty
    When I add a ticket inside that group
    Then the ticket belongs to that group
    And the group and membership remain after refreshing the page

  Scenario: Groups follow individual tickets
    Given a Todo ticket belongs to a group
    When I drag the ticket into Doing
    Then its saved prompt starts the usual queued run
    And the ticket retains its group name and colour
    When the run completes successfully
    Then the ticket appears in that group in Done

  Scenario: Minimise a group
    Given a group is expanded in a lane
    When I minimise it
    Then its tickets are hidden in that lane
    And its name and ticket count remain visible
    When I expand it
    Then its tickets are visible again

  Scenario: Edit or remove a group
    When I edit a group's name and colour through its menu
    Then the changes appear across every lane containing that group
    When I remove the group
    Then its tickets become ungrouped
    And their prompts, responses, files and run history are preserved

  Scenario: Organise completed work
    Given a completed ticket has a response and changed files
    When I change its group in the ticket detail pane
    Then its membership is updated
    And its prompt remains locked
    And its original run snapshot is unchanged

  Scenario: Custom group colour
    When I choose a colour swatch or use the custom colour picker
    And I save the group
    Then the group uses the chosen colour across its lanes
    And custom colours remain after refreshing the page
    And the header uses contrasting light or dark text
    And minimise and menu controls are labelled circular icon buttons

Feature: Run a group of saved prompts
  Scenario: Run from top to bottom
    Given a group contains saved Todo tickets
    When I choose Run group from its menu
    Then all eligible prompts are snapshotted and queued in board order
    And each next prompt starts only after its predecessor succeeds
    And every real-agent predecessor must have passing configured tests
    And demo test outcomes are labelled simulated

  Scenario: Stop when a prompt fails or is cancelled
    Given a group sequence has started
    When an attempt fails, times out, fails its tests or is cancelled
    Then no following prompt in that sequence executes
    And following attempts are marked Cancelled with a group-stopped explanation
    And an explicit retry is required to start an attempt again

  Scenario: Validate the entire group before starting
    Given a group has a real-agent ticket without enabled tests or a test preset
    When I choose Run group
    Then I see an explanation
    And no tickets in that group are queued
    And the same group cannot be started while any of its tickets are queued or running

  Scenario: Preserve sequence dependencies across restart
    Given a group sequence is queued
    When the backend restarts
    Then queued dependencies remain saved
    And an interrupted predecessor prevents following prompts from executing

  Scenario: Follow ticket progress in a group
    Given a group sequence is running
    Then queued tickets show an empty waiting progress bar
    And the running ticket shows an indeterminate animated progress bar
    And no estimated percentage is claimed
    When a ticket completes successfully
    Then it shows a green tick inside a circle
    And its progress bar is removed
    And failed or cancelled tickets never show the success tick
    And reduced-motion preferences stop the progress animation

Feature: Drag tickets between groups
  Scenario: Drag tickets between groups
    Given I have two groups and an ungrouped ticket
    When I drag the ticket onto a group header or its cards
    Then the ticket joins that group
    When I drag it onto another group, even if collapsed
    Then it moves to that group
    When I drag it anywhere in its current column outside a group
    Then it becomes ungrouped
    And its status, response, files and run history stay unchanged
    And unsaved prompt edits stay intact
    And group drops never trigger a run
    And there is no separate remove-from-group drop area
    And the membership persists after reloading

Feature: Keep queue results current
  Scenario: Live connection stops delivering events without disconnecting
    Given the live event connection is open but delivers no updates
    When I queue two tickets
    Then they execute sequentially
    And the board and selected ticket refresh at least every two seconds
    And completed tickets move to Done without reloading the page
    And the selected ticket shows its response and completed run history

Feature: Describe the current project
  Scenario: Edit project title and description
    When I choose Edit project from the board header
    And I save a title and description
    Then the board header shows the project title and description
    And the breadcrumb shows the project title
    And the fields remain after a page refresh or backend restart

  Scenario: Add a project description
    Given the project has no description
    Then the header offers Add a project description
    When I use that action
    Then I can add a description in the project dialog
    And an empty project title cannot be saved

  Scenario: Styled status filtering
    When I open Filter status
    Then its menu matches the group menu styling
    And I can select a status using the keyboard
    And Escape dismisses it and restores focus to the trigger
    And the search input has horizontal padding

Feature: Manage projects and delete tickets
  Scenario: Switch projects using the breadcrumb dropdown
    Given I am viewing a board or the projects page
    When I click the project dropdown in the breadcrumb
    Then all active projects are listed
    And the current project is highlighted and checked
    And deleted projects are managed through the projects page
    When I select a project
    Then its board opens
    And unsaved ticket changes require an explicit discard before switching projects
    And Escape dismisses the dropdown
    And Manage projects opens the projects page

  Scenario: Select a range of tickets
    Given tickets are displayed on the board
    When I click the first ticket and Shift-click the last ticket
    Then all displayed tickets between them are selected inclusively
    And ranges follow column and group display order
    And tickets hidden by filters or collapsed groups are excluded from the range

  Scenario: Select individual tickets and confirm bulk deletion
    Given tickets are displayed on the board
    When I Ctrl-click tickets or Cmd-click them on Mac
    Then each clicked ticket toggles its selection
    And a bottom action bubble shows the selected count and Delete all
    When I choose Delete all
    Then a confirmation lists the selected tickets and explains how to restore them
    And unsaved edits to a selected ticket show a discard warning
    When I cancel
    Then no tickets are deleted and the selection remains
    When I confirm deletion
    Then all selected tickets move to Deleted tickets together
    And the selection clears
    And queued or running tickets prevent the entire deletion
    And changing projects or filters clears selection

  Scenario: Open projects from the home logo
    Given I am viewing a project board
    When I click the top-left logo
    Then the projects page opens without reloading the app
    And unsaved ticket changes require an explicit discard before it opens

  Scenario: Open projects from Workspace and save a screenshot
    Given I am viewing a project board
    When I click Workspace in the breadcrumb
    Then I see the projects page instead of the board
    And project cards show their title, description and screenshot when available
    When I create or edit a project
    Then I can upload or paste one PNG, JPEG or WebP screenshot up to 5 MB
    And I can enlarge or remove the screenshot before saving
    And unsupported images show a validation error
    And the saved screenshot persists after reload, deletion and restore
    When I open a project card
    Then its board opens

  Scenario: Create and switch between isolated project boards
    Given my existing board belongs to My first project
    When I open Manage projects from the project name in the breadcrumb
    And I create a project with a title and description
    Then the new project has an empty board
    And tickets and groups created there belong only to that project
    And I can edit its title and description
    And I can open another project without seeing the previous project's tickets
    And the selected project remains selected after reloading

  Scenario: Delete and restore a ticket
    Given a ticket is not queued or running
    When I choose Delete ticket in its details header
    Then it disappears from its board
    And it appears in Deleted tickets in project management
    When I restore it
    Then its prompt, status, response, images, diffs and run history remain intact
    And it becomes ungrouped if its old group was removed

  Scenario: Delete and restore a project
    Given a project has no queued or running tickets
    When I choose Delete for that project in Manage projects
    Then the project and its board disappear from active projects
    And another available project opens
    And deleting the final project shows an empty workspace with creation and restore actions
    When I restore the project
    Then its title, description, tickets, groups and previous results are available again

  Scenario: Protect work in progress
    Given a ticket is queued or running
    Then its delete button is disabled
    And the API rejects deletion of the ticket or its project
    And I must cancel the run before deleting

  Scenario: Protect an unsaved ticket draft while managing projects
    Given my selected ticket or new draft has unsaved changes
    When I open project management
    Then I can keep editing or explicitly discard the changes
    And no project switch silently discards the draft
