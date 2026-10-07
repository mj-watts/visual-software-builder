Feature: Interchangeable local backends
  Scenario: Use Go while retaining the Python implementation
    Given the original Python backend and its SQLite data are present
    When I start the Go backend with a one-time import
    Then projects, tickets, groups, deleted items and run history are preserved
    And the Python database is not modified
    And an existing Go database is never overwritten by a later import
    And the frontend uses the Go backend on port 8080 by default

  Scenario: Preserve execution behaviour across backends
    Given a group contains saved Todo tickets
    When I run the group using the Go backend
    Then each prompt runs sequentially in board order
    And real-agent followers run only after preceding tests pass
    And failures or cancellation stop the remaining prompts
    And response, changed files and live events remain reviewable

  Scenario: Choose the original Python service
    Given the Python backend is running on port 8000
    When I start the frontend with SWIMLANE_API_URL pointing to that backend
    Then the existing board workflows use the Python backend
