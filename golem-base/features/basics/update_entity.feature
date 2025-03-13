Feature: update entity

  Scenario: updating the payload of the entity
    Given I have created an entity
    When I submit a transaction to update the entity, changing the paylod
    Then the payload of the entity should be changed

  Scenario: updating the annotations of the entity
    Given I have created an entity
    When I submit a transaction to update the entity, changing the annotations
    Then the annotations of the entity should be changed

  Scenario: updating the ttl of the entity
    Given I have created an entity
    When I submit a transaction to update the entity, changing the ttl of the entity
    Then the ttl of the entity should be changed
