classdef test_pcodable < matlab.unittest.TestCase
    % Copyright 2026 The MathWorks, Inc.

    % Test suite for basic math functions, which can be compiled to P-code (.p).
    % MATLAB can run P-coded test files that are class-based tests
    % (subclasses of matlab.unittest.TestCase)

    methods (Test)
        function testAddition(testCase)
            % Test basic addition
            testCase.verifyEqual(2 + 2, 4);
        end

        function testMultiplication(testCase)
            % Test basic multiplication
            testCase.verifyEqual(3 * 4, 12);
        end

        function testSquareRoot(testCase)
            % Test square root function
            testCase.verifyEqual(sqrt(16), 4);
        end

        function testMatrixMultiplication(testCase)
            % Test matrix multiplication (validates array operations)
            A = [1 2; 3 4];
            B = [2 0; 1 2];
            testCase.verifyEqual(A * B, [4 4; 10 8]);
        end

        function testMean(testCase)
            % Test statistical function
            testCase.verifyEqual(mean([1, 2, 3, 4, 5]), 3);
        end

        function testVectorizedOperations(testCase)
            % Test vectorized operations (common in MATLAB)
            testCase.verifyEqual((1:5).^2, [1, 4, 9, 16, 25]);
        end

        function testLogicalIndexing(testCase)
            % Test logical indexing (MATLAB-specific feature)
            data = [1, -2, 3, -4, 5];
            testCase.verifyEqual(data(data > 0), [1, 3, 5]);
        end
    end
end
