Feature: creating entities

  Scenario: creating an entity
    Given I have enough funds to pay for the transaction
    When submit a transaction to create an entity
    Then the entity should be created
    And the number of entities should be 1
    And the expiry of the entity should be recorded
    And the entity should be in the list of all entities
    And the sender should be the owner of the entity
    And the entity should be in the list of entities of the owner
    And I should be able to see the entity in the RESTful API
    And the entity should be in the list of entities of the owner in the RESTful API
