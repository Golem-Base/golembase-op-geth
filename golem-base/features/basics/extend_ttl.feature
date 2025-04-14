Feature: Extend TTL

  Scenario: Extend TTL
    Given I have created an entity
    When I submit a transaction to extend TTL of the entity by 100 blocks    
    Then the entity's TTL should be extended by 100 blocks
