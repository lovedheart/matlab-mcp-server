classdef DefaultConnectionMonitorTest < matlab.mock.TestCase
    %DefaultConnectionMonitorTest Tests for mcpserver.internal.connection.DefaultConnectionMonitor
    %   Verifies the beacon polling state machine: the monitor transitions
    %   to connected when a matching agent-connected beacon appears and
    %   back to disconnected when it disappears.

    % Copyright 2026 The MathWorks, Inc.

    properties(Access = private)
        MockFSFacade
        FSFacadeBehavior
        MockTimerFactory
        TimerFactoryBehavior
        MockTimer
        TimerBehavior
    end

    methods(TestMethodSetup)
        function createMocks(testCase)
            [testCase.MockFSFacade, testCase.FSFacadeBehavior] = testCase.createMock( ...
                ?mcpserver.internal.facade.fs.FSFacade);
            [testCase.MockTimerFactory, testCase.TimerFactoryBehavior] = testCase.createMock( ...
                ?mcpserver.internal.facade.timer.TimerFactoryFacade);
            [testCase.MockTimer, testCase.TimerBehavior] = testCase.createMock( ...
                ?mcpserver.internal.facade.timer.TimerFacade);
        end
    end

    methods(Test)
        function testEngageStartsTimerOnce(testCase)
            % Arrange
            import matlab.mock.actions.AssignOutputs
            import matlab.mock.actions.DoNothing

            monitor = mcpserver.internal.connection.DefaultConnectionMonitor( ...
                FSFacade=testCase.MockFSFacade, ...
                TimerFactory=testCase.MockTimerFactory);

            timerQuery = testCase.TimerFactoryBehavior.timer().withAnyInputs();
            when(timerQuery, AssignOutputs(testCase.MockTimer));
            startQuery = testCase.TimerBehavior.Start().withExactInputs();
            when(startQuery, DoNothing);

            % Act - engage twice; the monitor must stay usable and idempotent
            monitor.engage("details.json", "beacon.json", "session-1");
            monitor.engage("details.json", "beacon.json", "session-1");

            % Assert
            testCase.verifyCalled(timerQuery, ...
                "a timer should be created when the monitor engages");
            testCase.verifyCalled(startQuery, ...
                "the timer should be started when the monitor engages");
            testCase.verifyFalse(monitor.IsConnected);
        end

        function testPollConnectedThenDisconnected(testCase)
            % Arrange
            import matlab.mock.actions.AssignOutputs
            import matlab.mock.actions.DoNothing

            monitor = mcpserver.internal.connection.DefaultConnectionMonitor( ...
                FSFacade=testCase.MockFSFacade, ...
                TimerFactory=testCase.MockTimerFactory);

            beaconPath = "beacon.json";
            beaconJSON = '{"sessionId":"session-1","clientName":"Test Client","connectedAt":"2026-01-01T00:00:00Z"}';

            when(testCase.TimerFactoryBehavior.timer().withAnyInputs(), ...
                AssignOutputs(testCase.MockTimer));
            when(testCase.TimerBehavior.Start().withExactInputs(), DoNothing);

            monitor.engage("details.json", beaconPath, "session-1");

            when(testCase.FSFacadeBehavior.isfile(beaconPath), AssignOutputs(true));
            fopenQuery = testCase.FSFacadeBehavior.fopen(beaconPath, 'r');
            when(fopenQuery, AssignOutputs(3, ""));
            freadQuery = testCase.FSFacadeBehavior.fread(3, Inf, 'char=>char');
            when(freadQuery, AssignOutputs(string(beaconJSON)));
            fcloseQuery = testCase.FSFacadeBehavior.fclose(3);
            when(fcloseQuery, DoNothing);

            % Act - beacon appears: connect
            monitor.poll();

            % Assert
            testCase.verifyTrue(monitor.IsConnected);

            % Arrange - beacon disappears
            when(testCase.FSFacadeBehavior.isfile(beaconPath), AssignOutputs(false));

            % Act
            monitor.poll();

            % Assert
            testCase.verifyFalse(monitor.IsConnected);
        end

        function testPollBeaconForOtherSessionIsIgnored(testCase)
            % Arrange
            import matlab.mock.actions.AssignOutputs
            import matlab.mock.actions.DoNothing

            monitor = mcpserver.internal.connection.DefaultConnectionMonitor( ...
                FSFacade=testCase.MockFSFacade, ...
                TimerFactory=testCase.MockTimerFactory);

            beaconPath = "beacon.json";
            beaconJSON = '{"sessionId":"other-session","clientName":"Test Client"}';

            when(testCase.TimerFactoryBehavior.timer().withAnyInputs(), ...
                AssignOutputs(testCase.MockTimer));
            when(testCase.TimerBehavior.Start().withExactInputs(), DoNothing);

            monitor.engage("details.json", beaconPath, "session-1");

            when(testCase.FSFacadeBehavior.isfile(beaconPath), AssignOutputs(true));
            fopenQuery = testCase.FSFacadeBehavior.fopen(beaconPath, 'r');
            when(fopenQuery, AssignOutputs(3, ""));
            freadQuery = testCase.FSFacadeBehavior.fread(3, Inf, 'char=>char');
            when(freadQuery, AssignOutputs(string(beaconJSON)));
            fcloseQuery = testCase.FSFacadeBehavior.fclose(3);
            when(fcloseQuery, DoNothing);

            % Act
            monitor.poll();

            % Assert - beacon belongs to a different session
            testCase.verifyFalse(monitor.IsConnected);
        end

        function testPollStaleHeartbeatBeaconIsIgnored(testCase)
            % Arrange
            import matlab.mock.actions.AssignOutputs
            import matlab.mock.actions.DoNothing

            monitor = mcpserver.internal.connection.DefaultConnectionMonitor( ...
                FSFacade=testCase.MockFSFacade, ...
                TimerFactory=testCase.MockTimerFactory);

            beaconPath = "beacon.json";
            staleUnix = posixtime(datetime("now", TimeZone="UTC")) - 1000;
            beaconJSON = sprintf('{"sessionId":"session-1","clientName":"Test Client","heartbeatAtUnix":%.6f}', staleUnix);

            when(testCase.TimerFactoryBehavior.timer().withAnyInputs(), ...
                AssignOutputs(testCase.MockTimer));
            when(testCase.TimerBehavior.Start().withExactInputs(), DoNothing);

            monitor.engage("details.json", beaconPath, "session-1");

            when(testCase.FSFacadeBehavior.isfile(beaconPath), AssignOutputs(true));
            fopenQuery = testCase.FSFacadeBehavior.fopen(beaconPath, 'r');
            when(fopenQuery, AssignOutputs(3, ""));
            freadQuery = testCase.FSFacadeBehavior.fread(3, Inf, 'char=>char');
            when(freadQuery, AssignOutputs(string(beaconJSON)));
            fcloseQuery = testCase.FSFacadeBehavior.fclose(3);
            when(fcloseQuery, DoNothing);

            % Act
            monitor.poll();

            % Assert - heartbeat too old: treat as not connected
            testCase.verifyFalse(monitor.IsConnected);
        end

        function testPollCorruptBeaconIsIgnored(testCase)
            % Arrange
            import matlab.mock.actions.AssignOutputs
            import matlab.mock.actions.DoNothing

            monitor = mcpserver.internal.connection.DefaultConnectionMonitor( ...
                FSFacade=testCase.MockFSFacade, ...
                TimerFactory=testCase.MockTimerFactory);

            beaconPath = "beacon.json";

            when(testCase.TimerFactoryBehavior.timer().withAnyInputs(), ...
                AssignOutputs(testCase.MockTimer));
            when(testCase.TimerBehavior.Start().withExactInputs(), DoNothing);

            monitor.engage("details.json", beaconPath, "session-1");

            when(testCase.FSFacadeBehavior.isfile(beaconPath), AssignOutputs(true));
            fopenQuery = testCase.FSFacadeBehavior.fopen(beaconPath, 'r');
            when(fopenQuery, AssignOutputs(3, ""));
            freadQuery = testCase.FSFacadeBehavior.fread(3, Inf, 'char=>char');
            when(freadQuery, AssignOutputs(string("not valid json")));
            fcloseQuery = testCase.FSFacadeBehavior.fclose(3);
            when(fcloseQuery, DoNothing);

            % Act
            monitor.poll();

            % Assert
            testCase.verifyFalse(monitor.IsConnected);
        end
    end

end
