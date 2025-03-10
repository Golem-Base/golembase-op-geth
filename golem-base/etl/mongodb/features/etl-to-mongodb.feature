@wip
Feature: ETL to Mongodb

  
  Scenario: ETL Create to Mongodb
    Given A running Golembase node with WAL enabled
    And A running ETL to Mongodb
    When I create a new entity in Golebase
    Then the entity should be created in the Mongodb database
    And the annotations of the entity should be existing in the Mongodb database

   Scenario: ETL Update to Mongodb
    Given A running Golembase node with WAL enabled
    And A running ETL to Mongodb
    And an existing entity in the Mongodb database
    When update the entity in Golembase
    Then the entity should be updated in the Mongodb database
    And the annotations of the entity should be updated in the Mongodb database

   Scenario: ETL Delete to Mongodb
    Given A running Golembase node with WAL enabled
    And A running ETL to Mongodb
    And an existing entity in the Mongodb database
    When delete the entity in Golembase
    Then the entity should be deleted in the Mongodb database
