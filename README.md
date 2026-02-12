# Deployd

deploy golang service as ubuntu systemd service

Test deploy

// TODO: node selection at first job deployment; and then save the selection for subsequent deploy
// save the target deployment host (odd number; 3 or 5)
// determince which is the "leader" (for raft aware app ofcourse)

// mutable accross many deployment (eg. shared state each deployment job); for job manager; one service -> one service "stateinstance"; 
// the one that can be modified by user is "service definition"; the one we're talking is the "service instance" with 1 to 1 relationship.

// app can propose (eg. to publish raft nodehost port), via job manager of course

// Job manager also can have this storage:
 - host - available port mapping;
 - service -> port used mapping;

// Job manager may also use this for healthcheck etc.
