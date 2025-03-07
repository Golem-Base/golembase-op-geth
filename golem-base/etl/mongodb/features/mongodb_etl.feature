Feature: MongoDB ETL
  As a developer
  I want to ensure the MongoDB ETL works correctly
  So that data is correctly persisted to MongoDB

  Scenario: Basic MongoDB operations
    Given I have a MongoDB database
    When I insert a test entity
    Then I can retrieve the test entity
    When I insert a test annotation
    Then I can retrieve the test annotation
    When I delete the test entity
    Then the test entity should be gone 