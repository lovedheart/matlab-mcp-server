classdef DesktopTitle
    %DesktopTitle Read/write the MATLAB desktop title
    %   Supports the JavaScript Desktop API (R2025a and later) and falls
    %   back to the Java/Swing desktop on older releases. Both operations
    %   return an ok flag instead of throwing so callers can degrade
    %   gracefully in headless environments.

    % Copyright 2026 The MathWorks, Inc.

    methods (Static)
        function [ok, title] = read()
            title = "";

            try
                app = matlab.ui.container.internal.RootApp.getInstance();
                title = string(app.Title);
                ok = true;
                return;
            catch
            end

            try
                frame = com.mathworks.mlservices.MatlabDesktopServices.getDesktop().getMainFrame();
                title = string(char(frame.getTitle()));
                ok = true;
            catch
                ok = false;
            end
        end

        function ok = write(title)
            try
                app = matlab.ui.container.internal.RootApp.getInstance();
                [~] = subsasgn(app, substruct('.','Title'), char(title));
                ok = true;
                return;
            catch
            end

            try
                frame = com.mathworks.mlservices.MatlabDesktopServices.getDesktop().getMainFrame();
                frame.setTitle(char(title));
                ok = true;
            catch
                ok = false;
            end
        end
    end

end
