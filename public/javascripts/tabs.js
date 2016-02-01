/* ====================== MAINPAGE  ===================== */
/* THIS IS THE JAVASCRIPT FOR THE MAIN PAGE */

function checkUser(username) {
  console.log('checking');
    return username === "Guest";
}

function submitForm(username, type, updateTime) {
  if (checkUser(username)) {
    alert('Must be logged in to submit');
    return false;
  }
  return checkDB(type, updateTime);
}


//Nav bar active state
$(".nav a").on("click", function(){
   $(".nav").find(".active").removeClass("active");
   $(this).parent().addClass("active");
});

$(function() {
  $('.navigation a').on('click', function() {
    var tab = $(this).attr("href").substring(1);
    var subpage = window.location.href.slice(window.location.href.lastIndexOf('/'));
    var url = removeQuery('tab', subpage)
    url = addQuery('tab', tab, url);
    window.history.replaceState('', document.title, url);
    setActiveTab(window.location.href);
    setWindowHeight();
  });
});

// Function to fix window height
// The main container (white div) will be at a minimum the height of the window size
// If the container inside (the table) overflows, it will resize to fit.
// Plus an offset of 300px to account for the header
function setWindowHeight() {
  var container = $('.wrap');
  var inner = $('.tab-pane.active');
  if(inner.height() > $(window).innerHeight()) {
    container.height(inner.height()+300);
  } else {
    $('.container.wrap').css({ height: $(window).innerHeight() });
    $(window).resize(function(){
      $('.container.wrap').css({ height: $(window).innerHeight() });
    });
  }
}

// Function to remove query, i.e if questions are filtered by tag, user, URL, keyword etc.
// URL splits on any query (?) and the & to maintain the currently selected tab.
function removePageQuery(queryString) {
  console.log(queryString);
  var sourceURL = window.location.href;
  var queries = sourceURL.split(queryString);
  var tab = sourceURL.split('&')[1];
  console.log(queries[0]+'?'+tab);
  window.location = queries[0]+'?'+tab;
}

//---------- SET ACTIVE TAB --------------//
// removes the currently active tab, and then finds the new one based on the URL
// resizes the container div height
function setActiveTab(sourceURL) {
  $('.active').removeClass('active');
  var queries = sourceURL.slice(window.location.href.indexOf('?')+1).split('&');
  var param;
  for (var i = 0; i < queries.length; i++) {
    var urlparam = queries[i].split('=');
    if (urlparam[0] == 'tab') {
      param = '#' + urlparam[1];
      break;
    }
  }
  $('li a[href="'+param+'"]').parent().addClass('active');
  $(param).addClass('active');
  setWindowHeight();
}

//---------- CLEAR SELECTION BUTTON -------------- //
$(function() {
  $('#clearButton').on('click', function() {
    $('.new_state_menu').val('no_change');
  });
});

//---------- SUBMIT BUTTON RELOAD PAGE ----------- //
function checkDB(buttonPressed, updateTime) {
  $.post('/dbUpdated?time='+updateTime, function( dataJSON ) {
    var title;
    if (buttonPressed == 'submit') {
      title = $('.new_state_menu option[value!="no_change"]:selected').parent().parent().parent().siblings('.question').children('.question_title').text();
    } else if (buttonPressed != '') {
      title = $('.one-click.clicked').parent().siblings('.question').children('.question_title').text();
      $('.new_state_menu').val('no_change');
      $('.one-click.clicked').siblings('.new_state_menu').val(buttonPressed);
    }

    var data = JSON.parse(dataJSON);
    if (data.Updated) {
      // Writing the text to display in the confirm dialog box
      confirm_text = 'Database has been updated\nChanges:\n';
      for (i = 0; i < data.Questions.length; i++) {
        if (data.Questions[i] === title) {
          confirm_text += '\t* ' + title + '\n';
          break;
        }
      }

      confirm_text += '\nDo you wish to continue submit?';
      if (confirm_text.indexOf('*') > -1) {
        if (!confirm(confirm_text)) {
          $('form select').val('no_change');
          return;
        }
      }
    }
    $('#stateForm').submit();
  });
}

// Function to remove any content from the question table
// If the user has previewed a question, and either cancelled its submission
// or if they have previewed a question and want to preview a new one the table needs to be
// cleared so content doesn't stack on top of itself

function clearTextPreserveChildren(element) {
  element.contents().filter(function() {
    return(this.nodeType == 3);
  }).remove();
}


// Function to manually pull a question from StackOverflow using it's ID or URL
// Checks that the input from the form is either a URL or an id number
// Parses the incoming string to isolate the question ID from the URL
// Uses the result as a parameter in a call to Stack Exchange API
// The response is returned as a JSON object, which is then inserted into the page
function pullQuestionFromStackOverflow() {
  var query = $('input#searchTerm').val();

  if(!$.isNumeric(query)) {
    var check1 = canSplit(query, "http://stackoverflow.com/questions/");

    if(check1) {
      var queryA = query.split("http://stackoverflow.com/questions/");
      var check2 = canSplit(query.split("http://stackoverflow.com/questions/")[1], "/");

      if(check2) {
        queryB = query.split("http://stackoverflow.com/questions/")[1].split("/")[0];

        if($.isNumeric(queryB)) {
          pullNewQn(queryB);   
        }
      } else {
        alert("Not a valid question URL or ID. Please try again");
      }
    }
  } else {
    pullNewQn(query);
  }
}

function pullNewQn(query) {
  $.post('/pullNewQn?id='+query, function( data ) {
            //Clear the text from the table
            var table = $('table');
            clearTextPreserveChildren(table);
            displayNewQuestion(data);
          });
}


// Parses a JSON object and displays it in the table for viewing
// First, it determines if the data is a new or existing question
// new/existing require different button functionality and different naming
// A previously existing question has an extra field called Message, which 
// basically says that this is an existing question.
// This means this question must have some sort of state assigned to it
// It needs to display this state, and the appropriate buttons for usage
// The select menu is only shown on questions marked as unanswered, pending or
// updating. The answered state only has a single button marked Reopen.
// 
// The JSON is saved in local storage, so that if that question is submitted 
// into the database the question information is still available
function displayNewQuestion(data) {
  var question = JSON.parse(data);
  var type = question.Message;
  var btn = $('.function-button');
  var menu = $('.new_state_menu');
  var cancel = $('.cancel-button');
  menu.empty();

  btn.attr('name', 'unanswered_'+ question.Question_id);
  btn.click().addClass('clicked');
  btn.attr('value', 'Pending');

  if(type == undefined) {
    menu.removeClass('hidden');
    cancel.removeClass('hidden');
    options = {
      "unanswered":"Unanswered", 
      "answered":"Answered",
      "pending":"Pending",
      "updating":"Updating"
    };
    btn.click(function() {
      addQuestionToStackTracker(data, btn.attr('value').toLowerCase());
    });
    //menu.change(addQuestionToStackTracker(data));
  }

  $('.questionExists').html(question.Message);
  $('table').removeClass('hidden');
  $('a.question_title').attr("href", question.Link).children('h4').html(((question.Title)));
  $('table td.question .bodySnippet').html(question.Body);
  $('ul.tags').empty();
  $.each(question.Tags, function(i, item) {
    $('.tagContainer ul.tags').append('<a href="/tag?tagSearch='+item+'"><li class="tag">'+item+'</li></a>');
  });
}

// Function to save aspects of the reply in local storage to use in jquery
// TODO: add update time
function saveState(user) {
  localStorage["currentUser"] = user;
  localStorage["lastUpdateTime"] = $.now();
  console.log("saved state of", user, ", ", localStorage["lastUpdateTime"]);
}


// Helper function to ensure input validation on the add question field
// If the user enters something that cannot be recognised, it should return false
 function canSplit(str, token) {
  return(str || '').split(token).length > 1;
 }

// Function to post to server to add the new question to StackTrackers database
// newQuestion is the stringified JSON data that is cached in localStorage
function addQuestionToStackTracker(newQuestion, newState) {
  var data = {"question": newQuestion, "state": newState};
  $.ajax({
    type: "POST",
    url: "/addNewQuestion",
    processData: false,
    contentType: 'application/json',
    data: JSON.stringify({"Question":newQuestion, "State":newState}),
    success: function( data ) {
      
    }
  });
}

$(function() {
  $('#stateForm').submit(function() {
    $('#submitButton').prop('value', 'Submitting');
    $('#submitButton').prop('disabled', true);
    document.cookie = 'submitting=true';

    // Intercept form submission and redirect back to the original page
    $.post( '/', $('#stateForm').serialize()).done(function( data ) {
      window.location = window.location.href.split('#')[0];
    });
    return false;
  });
});

//-------- SETTING COOKIES -------//
$(document).ready(function() {
  setCookies();
  clearEmptyQueries();

  var subpage = window.location.href.split('?')[0].slice(window.location.href.lastIndexOf('/'));
  if (window.location.search.indexOf('tab') == -1 &&
      subpage.indexOf('viewTags') == -1 && subpage.indexOf('viewUsers') == -1 &&
      subpage.indexOf('addQuestion') == -1) {
      var addedPath = subpage + addQuery('tab', 'unanswered', window.location.search);
    window.history.replaceState('', document.title, addedPath);
  } else if (window.location.search.indexOf('page') == -1 && (subpage.indexOf('viewTags') != -1 ||
             subpage.indexOf('viewUsers') != -1)) {
    var addedPath = subpage + addQuery('page', '1', window.location.search);
    window.history.replaceState('', document.title, addedPath);
  }
  setActiveTab(window.location.href);
});

//-------- COOKIES --------//
function setCookies() {
  var code = location.search.split('code=')[1];
  if (code !== undefined && code !== '') {
    document.cookie = 'code=' + code;
  }
  var url = removeQuery('code', window.location.href);
  window.history.replaceState("", document.title, url);
}

//-------- Removing queries ---------//
function clearEmptyQueries() {
  var urlParts = window.location.href.split('?');
  var newURL = urlParts[0].slice(window.location.href.lastIndexOf('/'));
  var param;
  var params_arr = [];
  var queryString = urlParts[1];

  if (typeof queryString !== 'undefined') {
    params_arr = queryString.split('&');
    for (var i = params_arr.length -1; i >= 0; i--) {
      query = params_arr[i].split('=')[1];
      if (query == '') {
        params_arr.splice(i, 1);
      }
    }
    newURL += '?' + params_arr.join('&');
  }
  window.history.replaceState('', document.title, newURL);
}

// returns path including modified query assuming sourceURL does not
// contain the key
function addQuery(key, query, sourceURL) {
  var newURL = sourceURL;
  if (sourceURL.indexOf('?') !== -1) {
    if (sourceURL.split('?')[1] !== '') {
      newURL += '&';
    }
  } else {
    newURL += '?';
  }
  newURL += key + '=' + query;
  return newURL;
}

// returns path without query specified
function removeQuery(key, sourceURL) {
  var newURL = sourceURL.split('?')[0];
  var param;
  var params_arr = [];
  var queryString = sourceURL.split('?')[1];

  if (typeof queryString !== 'undefined') {
    params_arr = queryString.split('&');
    for (var i = params_arr.length -1; i >= 0; i--) {
      param = params_arr[i].split('=')[0];
      if (param == key) {
        params_arr.splice(i, 1);
      }
    }
    if (params_arr.length > 0) {
      newURL += '?' + params_arr.join('&');
    }
  }
  return newURL;
}
