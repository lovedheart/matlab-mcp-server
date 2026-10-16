classdef DefaultSessionIdProvider < mcpserver.internal.connection.SessionIdProvider
    %DefaultSessionIdProvider Generates and caches a GUID for the MATLAB session
    %   The GUID is created on first use and cached in a persistent
    %   variable, so all shareMATLABSession calls within one MATLAB session
    %   report the same Session ID.

    % Copyright 2026 The MathWorks, Inc.

    methods
        function sessionID = getSessionId(~)
            persistent cachedSessionID
            if isempty(cachedSessionID)
                cachedSessionID = mcpserver.internal.connection.DefaultSessionIdProvider.generateGuid();
            end
            sessionID = cachedSessionID;
        end
    end

    methods (Static, Access = private)
        function guid = generateGuid()
            import mcpserver.internal.connection.DefaultSessionIdProvider

            try
                guid = string(char(java.util.UUID.randomUUID()));
            catch
                try
                    guid = string(char(System.Guid.NewGuid().ToString()));
                catch
                    guid = DefaultSessionIdProvider.generateRandomHex();
                end
            end
        end

        function guid = generateRandomHex()
            % Last-resort fallback: 32 random hex digits formatted like a GUID.
            digits = '0123456789abcdef';
            raw = digits(randi(numel(digits), 1, 32));
            parts = {raw(1:8), raw(9:12), raw(13:16), raw(17:20), raw(21:32)};
            guid = strjoin(parts, '-');
        end
    end

end
