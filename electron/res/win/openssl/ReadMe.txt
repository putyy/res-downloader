=============================================================================
OpenSSL v1.0.2q                                Precompiled Binaries for Win32
-----------------------------------------------------------------------------

                         *** Release Information ***

Release Date:     Nov 22, 2018

Author:           Frederik A. Winkelsdorf (opendec.wordpress.com)
                  for the Indy Project (www.indyproject.org)

Requirements:     Indy 10.5.5+ (SVN Version or Delphi 2009 and newer)

Dependencies:     The libraries have no noteworthy dependencies

Installation:     Copy both DLL files into your application directory

Supported OS:     Windows 2000 up to Windows 10

-----------------------------------------------------------------------------

                          *** Legal Disclaimer ***

THIS SOFTWARE IS PROVIDED BY ITS AUTHOR AND THE INDY PROJECT "AS IS" AND ANY 
EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED 
WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE 
DISCLAIMED. IN NO EVENT SHALL THE REGENTS OR CONTRIBUTORS BE LIABLE FOR ANY 
DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES 
(INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; 
LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND 
ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT 
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF 
THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

OpenSSL license terms are provided in the file "OpenSSL License.txt".

PLEASE CHECK IF YOU NEED TO COMPLY WITH EXPORT RESTRICTIONS FOR CRYPTOGRAPHIC
SOFTWARE AND/OR PATENTS.

-----------------------------------------------------------------------------

                       *** Build Information Win32 ***

Built with:       Microsoft Visual C++ 2008 Express Edition
                  The Netwide Assembler (NASM) v2.11.08 Win32
                  Strawberry Perl v5.22.0.1 Win32 Portable
                  Windows PowerShell
                  FinalBuilder 7

Commands:         perl configure VC-WIN32
                  ms\do_nasm
                  adjusted ms\ntdll.mak       (replaced "/MD" with "/MT")
                  adjusted ms\version32.rc    (Indy Information inserted)
                  nmake -f ms\ntdll.mak
                  nmake -f ms\ntdll.mak test
                  editbin.exe /rebase:base=0x11000000 libeay32.dll
                  editbin.exe /rebase:base=0x12000000 ssleay32.dll

=============================================================================