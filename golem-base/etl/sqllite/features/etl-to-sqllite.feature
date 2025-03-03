@wip
Feature: ETL to SQLite

  Scenario: ETL Create to SQLite
    Given A running Golembase node with WAL enabled
    And A running ETL to SQLite
    When I create a new entity in Golebase
    Then the entity should be created in the SQLite database
    And the annotations of the entity should be existing in the SQLite database

  