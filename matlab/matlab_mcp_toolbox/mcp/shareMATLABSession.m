function shareMATLABSession()
    %shareMATLABSession Share the current MATLAB session via MCP server
    %   This function enables sharing of the MATLAB session through the
    %   Model Context Protocol (MCP) server. It can be run in any number
    %   of MATLAB sessions; each shared session gets a unique Session ID.
    %   Once an agent connects through the MCP server, the MATLAB desktop
    %   title shows "Agent Connected (Session ID: ...)" and a message with
    %   the Session ID appears in the command window. When the agent
    %   disconnects, the title is restored.

    % Copyright 2026 The MathWorks, Inc.

    mcpserver.internal.shareMATLABSession();
end
