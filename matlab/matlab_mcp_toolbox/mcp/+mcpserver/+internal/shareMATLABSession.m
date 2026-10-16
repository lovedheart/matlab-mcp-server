function shareMATLABSession(options)
    %shareMATLABSession Share the current MATLAB session via MCP server
    %   This function enables sharing of the MATLAB session through the
    %   Model Context Protocol (MCP) server. The connection details are
    %   written to a per-session file so that any number of MATLAB
    %   sessions can be shared concurrently. A background monitor then
    %   periodically checks whether an agent has connected to this
    %   session and reflects the connection state (including the Session
    %   ID) in the MATLAB desktop title and the command window.

    % Copyright 2026 The MathWorks, Inc.

    arguments
        options.AppDataLocator(1, 1) mcpserver.internal.appdata.AppDataLocator = mcpserver.internal.appdata.DefaultAppDataLocator()
        options.FSAdaptor(1, 1) mcpserver.internal.fs.FSAdaptor = mcpserver.internal.fs.DefaultFSAdaptor()
        options.FSFacade(1, 1) mcpserver.internal.facade.fs.FSFacade = mcpserver.internal.facade.fs.DefaultFSFacade()
        options.ConnectorAdaptor(1, 1) mcpserver.internal.connector.ConnectorAdaptor = mcpserver.internal.connector.DefaultConnectorAdaptor()
        options.ConnectorFacade(1, 1) mcpserver.internal.facade.connector.ConnectorFacade = mcpserver.internal.facade.connector.DefaultConnectorFacade()
        options.SessionIdProvider(1, 1) mcpserver.internal.connection.SessionIdProvider = mcpserver.internal.connection.DefaultSessionIdProvider()
        options.ConnectionMonitor(1, 1) mcpserver.internal.connection.ConnectionMonitor = mcpserver.internal.connection.DefaultConnectionMonitor.getInstance()
    end

    options.ConnectorFacade.ensureServiceOn();

    appDataFolder = options.AppDataLocator.getAppDataFolder();
    options.FSAdaptor.ensureSecureFolder(appDataFolder);

    v1Folder = fullfile(appDataFolder, "v1");
    options.FSAdaptor.ensureSecureFolder(v1Folder);

    sessionID = options.SessionIdProvider.getSessionId();

    sessionDetails = options.ConnectorAdaptor.getConnectionDetails();
    sessionDetails.sessionId = sessionID;
    jsonText = jsonencode(sessionDetails, PrettyPrint=true);

    % Write a per-process file so multiple shared MATLAB sessions do not
    % overwrite each other, plus the canonical file for backwards
    % compatibility with servers that only know the single-session layout.
    sessionDetailsPath = fullfile(v1Folder, sprintf("sessionDetails_%d.json", sessionDetails.pid));
    options.FSAdaptor.ensureSecureFile(sessionDetailsPath);
    options.FSFacade.writelines(jsonText, sessionDetailsPath);

    legacySessionDetailsPath = fullfile(v1Folder, "sessionDetails.json");
    options.FSAdaptor.ensureSecureFile(legacySessionDetailsPath);
    options.FSFacade.writelines(jsonText, legacySessionDetailsPath);

    % Beacon files are written by the MCP server once it attaches to this
    % session; the monitor polls them every few seconds.
    beaconPaths = [ ...
        string(fullfile(v1Folder, sprintf("agentConnected_%d.json", sessionDetails.pid))), ...
        string(fullfile(v1Folder, "agentConnected.json"))];
    options.ConnectionMonitor.engage(sessionDetailsPath, beaconPaths, sessionID);
end
