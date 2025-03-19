Feature: Search

  Scenario: finding multiple entities by common string annotation
    Given I have an entity "e1" with string annotations:
      | foo | bar |
    And I have an entity "e2" with string annotations:
      | foo | bar |
    And I have an entity "e3" with string annotations:
      | foo | baz |
    When I search for entities with the string annotation "foo" equal to "bar"
    Then I should find 2 entities

  Scenario: finding multiple entities by common numeric annotation
    Given I have an entity "e1" with numeric annotations:
      | foo | 42 |
    And I have an entity "e2" with numeric annotations:
      | foo | 42 |
    And I have an entity "e3" with numeric annotations:
      | foo | 43 |
    When I search for entities with the numeric annotation "foo" equal to "42"
    Then I should find 2 entities

  Scenario: finding multiple entities with a complex query
    Given I have an entity "e1" with string annotations:
      | foo | bar |
    And I have an entity "e2" with string annotations:
      | foo | bar |
    And I have an entity "e3" with string annotations:
      | foo | baz |
    When I search for entities with the query
      """
      (foo = "bar" || foo = "baz") && foo = "bar"
      """
    Then I should find 2 entities
