classdef (Abstract) SessionIdProvider
    %SessionIdProvider Provides a stable identifier for the current MATLAB session
    %   The identifier is generated once per MATLAB session and stays the
    %   same for repeated shareMATLABSession calls.

    % Copyright 2026 The MathWorks, Inc.

    methods (Abstract)
        sessionID = getSessionId(obj)
    end

end
