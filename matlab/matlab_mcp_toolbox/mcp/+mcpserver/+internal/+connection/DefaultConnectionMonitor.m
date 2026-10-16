classdef DefaultConnectionMonitor < mcpserver.internal.connection.ConnectionMonitor & handle
    %DefaultConnectionMonitor Timer-based agent-connection monitor

    % Copyright 2026 The MathWorks, Inc.

    properties (Constant, Access = private)
        PollPeriodSeconds = 5
        %MaxBeaconAgeSeconds: a beacon whose heartbeat is older than this is
        %   considered stale (e.g. the MCP server crashed without removing it).
        MaxBeaconAgeSeconds = 30
        TitleSuffixPrefix = "  |  Agent Connected (Session ID: "
        TitleSuffixPattern = "\s*\|\s*Agent Connected(\s*\(Session ID:[^)]*\))?\s*$"
    end

    properties (GetAccess = private, SetAccess = immutable)
        FSFacade(1, 1) mcpserver.internal.facade.fs.FSFacade = mcpserver.internal.facade.fs.DefaultFSFacade()
        TimerFactory(1, 1) mcpserver.internal.facade.timer.TimerFactoryFacade = mcpserver.internal.facade.timer.DefaultTimerFactoryFacade()
    end

    properties (SetAccess = private)
        IsConnected logical = false
    end

    properties (Access = private)
        Timer
        SessionID(1, 1) string = ""
        SessionDetailsPath(1, 1) string = ""
        BeaconPaths(1, :) string = ""
        OriginalTitle(1, 1) string = ""
        HasOriginalTitle logical = false
        MessageShown logical = false
        IsPolling logical = false
    end

    methods
        function obj = DefaultConnectionMonitor(options)
            arguments
                options.?mcpserver.internal.connection.DefaultConnectionMonitor
            end

            for prop = string(fieldnames(options).')
                obj.(prop) = options.(prop);
            end
        end
    end

    methods (Static)
        function obj = getInstance()
            %getInstance Singleton accessor so repeated shareMATLABSession
            %   calls reuse one monitor (and therefore one timer).
            persistent singleton
            if isempty(singleton) || ~isvalid(singleton)
                singleton = mcpserver.internal.connection.DefaultConnectionMonitor();
            end
            obj = singleton;
        end
    end

    methods
        function engage(obj, sessionDetailsPath, beaconPaths, sessionID)
            arguments
                obj
                sessionDetailsPath (1, 1) string
                beaconPaths (1, :) string
                sessionID (1, 1) string
            end

            obj.SessionDetailsPath = sessionDetailsPath;
            obj.BeaconPaths = beaconPaths;
            obj.SessionID = sessionID;

            if isempty(obj.Timer)
                obj.Timer = obj.TimerFactory.timer( ...
                    obj.PollPeriodSeconds, 'fixedSpacing', 'drop', ...
                    @(~, ~) obj.poll());
                obj.Timer.Start();
            end
        end

        function poll(obj)
            %poll Check the agent-connected beacon and update UI state
            if obj.IsPolling
                return;
            end
            obj.IsPolling = true;
            cleanup = onCleanup(@() obj.releasePolling());

            [isConnected, beacon] = obj.readBeacon();

            if isConnected && ~obj.IsConnected
                obj.onAgentConnected(beacon);
            elseif ~isConnected && obj.IsConnected
                obj.onAgentDisconnected();
            end
        end
    end

    methods (Access = private)
        function releasePolling(obj)
            obj.IsPolling = false;
        end

        function [isConnected, beacon] = readBeacon(obj)
            isConnected = false;
            beacon = [];

            for idx = 1:numel(obj.BeaconPaths)
                beaconPath = obj.BeaconPaths(idx);
                if ~obj.FSFacade.isfile(beaconPath)
                    continue;
                end

                beacon = obj.readBeaconFile(beaconPath);
                if obj.isSessionBeacon(beacon)
                    isConnected = true;
                    return;
                end
            end
        end

        function valid = isSessionBeacon(obj, beacon)
            %isSessionBeacon Whether the decoded beacon describes this session
            %   and carries a fresh heartbeat.
            valid = false;
            try
                if ~isstruct(beacon) || ~isfield(beacon, 'sessionId') || ...
                        ~isequal(string(beacon.sessionId), obj.SessionID)
                    return;
                end
                valid = obj.beaconIsFresh(beacon);
            catch
                valid = false;
            end
        end

        function fresh = beaconIsFresh(obj, beacon)
            %beaconIsFresh A beacon counts as fresh while its heartbeat is
            %   recent; beacons without heartbeat information (older servers)
            %   are considered fresh as long as the file exists.
            fresh = true;
            if ~isfield(beacon, 'heartbeatAtUnix')
                return;
            end

            try
                nowUnix = posixtime(datetime("now", TimeZone="UTC"));
                heartbeat = double(beacon.heartbeatAtUnix);
                fresh = isscalar(heartbeat) && ...
                    (nowUnix - heartbeat) <= obj.MaxBeaconAgeSeconds;
            catch
                fresh = true;
            end
        end

        function beacon = readBeaconFile(obj, beaconPath)
            beacon = [];

            [fid, ~] = obj.FSFacade.fopen(beaconPath, 'r');
            if fid == -1
                return;
            end

            try
                raw = obj.FSFacade.fread(fid, Inf, 'char=>char');
                obj.FSFacade.fclose(fid);
                decode = jsondecode(char(raw.'));
                if isstruct(decode)
                    beacon = decode;
                end
            catch
                try
                    obj.FSFacade.fclose(fid);
                catch
                end
            end
        end

        function onAgentConnected(obj, beacon)
            obj.IsConnected = true;

            clientName = "";
            if isfield(beacon, 'clientName')
                clientName = strip(string(beacon.clientName));
            end
            if clientName == ""
                clientName = "An agent";
            end

            obj.applyConnectedTitle();

            if ~obj.MessageShown
                obj.displayConnectedMessage(clientName);
                obj.MessageShown = true;
            end
        end

        function onAgentDisconnected(obj)
            obj.IsConnected = false;
            obj.MessageShown = false;

            if obj.HasOriginalTitle
                obj.HasOriginalTitle = false;
                mcpserver.internal.connection.internal.DesktopTitle.write(obj.OriginalTitle);
                obj.OriginalTitle = "";
            end
        end

        function applyConnectedTitle(obj)
            [ok, currentTitle] = mcpserver.internal.connection.internal.DesktopTitle.read();
            if ~ok || currentTitle == ""
                obj.HasOriginalTitle = false;
                return;
            end

            obj.OriginalTitle = regexprep(currentTitle, obj.TitleSuffixPattern, "");
            obj.HasOriginalTitle = obj.OriginalTitle ~= "";
            if ~obj.HasOriginalTitle
                return;
            end

            suffix = obj.TitleSuffixPrefix + obj.SessionID + ")";
            mcpserver.internal.connection.internal.DesktopTitle.write(obj.OriginalTitle + suffix);
        end

        function displayConnectedMessage(obj, clientName)
            message = sprintf( ...
                '%s is connected to MATLAB via the MATLAB MCP server (Session ID: %s).\nNow you can use %s to work with MATLAB.', ...
                clientName, obj.SessionID, clientName);
            fprintf('\n%s\n', message);
        end

        function deleteSessionDetailsFile(obj)
            %deleteSessionDetailsFile Remove this session's details file
            %   Only deletes the file when it still belongs to this session,
            %   so stale files from earlier sessions are handled by the
            %   server while files of other live sessions are never touched.
            if isempty(obj.SessionDetailsPath) || obj.SessionDetailsPath == ""
                return;
            end

            try
                if ~isempty(obj.FSFacade.isfile(obj.SessionDetailsPath)) && ...
                        ~logical(obj.FSFacade.isfile(obj.SessionDetailsPath))
                    return;
                end

                [fid, ~] = obj.FSFacade.fopen(obj.SessionDetailsPath, 'r');
                if fid == -1
                    return;
                end
                raw = obj.FSFacade.fread(fid, Inf, 'char=>char');
                obj.FSFacade.fclose(fid);
                details = jsondecode(char(raw.'));
                if ~isstruct(details) || ~isfield(details, 'sessionId') || ...
                        ~isequal(string(details.sessionId), obj.SessionID)
                    return;
                end

                obj.FSFacade.delete(obj.SessionDetailsPath);
            catch
            end
        end
    end

    methods
        function delete(obj)
            %delete Stop polling and remove this session's details file
            if ~isempty(obj.Timer) && isvalid(obj.Timer)
                obj.Timer.Stop();
            end
            obj.deleteSessionDetailsFile();
        end
    end

end
