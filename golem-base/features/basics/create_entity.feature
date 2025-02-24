Feature: creating entities

  Scenario: creating an entity
    Given I have enough funds to pay for the transaction
    When submit a transaction to create an entity
    Then the entity should be created
    And the expiry of the entity should be recorded
