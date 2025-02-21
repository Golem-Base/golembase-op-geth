Feature: deleting entities

  Scenario: deleting an existing entity
    Given I have created an entity
    When I submit a transaction to delete the entity
    Then the entity should be deleted
