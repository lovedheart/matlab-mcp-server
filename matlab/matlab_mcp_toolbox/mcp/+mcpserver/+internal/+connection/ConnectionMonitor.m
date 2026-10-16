classdef (Abstract) ConnectionMonitor < handle
    %ConnectionMonitor Monitors whether an agent has connected to this session
    %   Once a MATLAB session is shared via shareMATLABSession, the
    %   monitor periodically checks whether the MCP server (and thereby an
    %   agent) has attached to it. When an agent connects, the monitor
    %   appends an "Agent Connected (Session ID: ...)" suffix to the MATLAB
    %   desktop title and displays a message in the command window. When the
    %   agent disconnects, the original title is restored.

    % Copyright 2026 The MathWorks, Inc.

    methods (Abstract)
        engage(obj, sessionDetailsPath, beaconPaths, sessionID)
    end

end
